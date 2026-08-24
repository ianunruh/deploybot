import { Alert, Button, Group, Stack, Text, TextInput, Select } from "@mantine/core";
import { useDisclosure } from "@mantine/hooks";
import { useEffect, useMemo, useState } from "react";
import { Link, useFetcher, useRevalidator } from "react-router";

import type { Route } from "./+types/deployables.$name";
import type { ImagesLoaderData } from "./deployables.$name.images";
import {
  getDeployable,
  getDeployableHistory,
  pinDeployable,
  promoteDeployable,
  type ImageVersion,
  type MutationResult,
  type UpdateStatus as RegistryUpdate,
} from "~/lib/api.server";
import { notifyActionError, notifyActionSuccess } from "~/lib/action-feedback";
import { formatAbsolute } from "~/lib/time";
import { useFetcherResult } from "~/lib/use-fetcher-result";
import { CompactImage, shortDigest } from "~/ui/compact-image";
import { ConfirmActionModal } from "~/ui/confirm-action-modal";
import { DiffPanel } from "~/ui/diff-panel";
import { ReleaseFlow } from "~/ui/release-flow";
import { ReleaseHistory } from "~/ui/release-history";
import { ReleaseTheater, type TheaterSession } from "~/ui/release-theater";
import {
  DeployableLinkLabels,
  HostnameLink,
  StageObservabilityIcons,
} from "~/ui/external-links";
import {
  ArgoSyncCheckbox,
  formFlag,
  MutationGitHint,
  mutationCommitLabel,
  mutationNote,
} from "~/ui/mutation-controls";
import { PageHeader } from "~/ui/page-header";
import { RelativeTime } from "~/ui/relative-time";
import { ResourceTable, Table } from "~/ui/resource-table";
import { StatusBadge, UpdateBadge } from "~/ui/status-badge";
import { StageReady, WorkloadPods } from "~/ui/workload-pods";
import { WorkflowRuns } from "~/ui/workflow-runs";

export function meta({ params }: Route.MetaArgs) {
  return [{ title: `${params.name} · deploybot` }];
}

export async function loader({ params }: Route.LoaderArgs) {
  const name = params.name;
  if (!name) throw new Response("Missing name", { status: 400 });
  const [statusResult, historyResult] = await Promise.allSettled([
    getDeployable(name),
    getDeployableHistory(name),
  ]);
  return {
    status: statusResult.status === "fulfilled" ? statusResult.value : null,
    error:
      statusResult.status === "rejected"
        ? statusResult.reason instanceof Error
          ? statusResult.reason.message
          : String(statusResult.reason)
        : null,
    history: historyResult.status === "fulfilled" ? historyResult.value : null,
    historyError:
      historyResult.status === "rejected"
        ? historyResult.reason instanceof Error
          ? historyResult.reason.message
          : String(historyResult.reason)
        : null,
  };
}

type ActionData =
  | { ok: true; intent: string; result?: MutationResult; diff?: string }
  | { ok: false; error: string };

export async function action({ request, params }: Route.ActionArgs) {
  const name = params.name;
  if (!name) {
    return { ok: false, error: "missing name" } satisfies ActionData;
  }
  const form = await request.formData();
  const intent = String(form.get("intent") ?? "");
  try {
    switch (intent) {
      case "pin":
        return {
          ok: true,
          intent,
          result: await pinDeployable(
            name,
            String(form.get("stage") ?? ""),
            String(form.get("image") ?? ""),
            { sync: formFlag(form, "sync"), wait: formFlag(form, "wait") },
          ),
        } satisfies ActionData;
      case "promote":
        return {
          ok: true,
          intent,
          result: await promoteDeployable(
            name,
            String(form.get("from") ?? ""),
            String(form.get("to") ?? ""),
            {
              sync: formFlag(form, "sync"),
              wait: formFlag(form, "wait"),
              image: String(form.get("image") ?? "") || undefined,
            },
          ),
        } satisfies ActionData;
      default:
        return { ok: false, error: `unknown intent ${intent}` } satisfies ActionData;
    }
  } catch (err) {
    return {
      ok: false,
      error: err instanceof Error ? err.message : String(err),
    } satisfies ActionData;
  }
}

const deployablesCrumb = { label: "Deployables", to: "/" };

function RegistryUpdateHint({ update }: { update: RegistryUpdate }) {
  const interval = update.auto ? ` Auto-pin ${update.auto}.` : "";
  let body: string;
  if (update.error) {
    body = `Registry check failed: ${update.error}`;
  } else if (update.stale && update.newest?.tag) {
    body = `${update.stage} is behind ${update.newest.tag}.${interval}`;
  } else if (update.newest) {
    body = `Up to date with registry.${interval}`;
  } else {
    body = `Tracking registry updates.${interval}`;
  }
  return (
    <Group gap="xs" wrap="nowrap">
      <UpdateBadge stale={update.stale} />
      <Text size="sm" c="dimmed">
        {body}
      </Text>
    </Group>
  );
}

export default function DeployableDetail({ loaderData }: Route.ComponentProps) {
  const { status, error, history, historyError } = loaderData;
  const revalidator = useRevalidator();
  const pinFetcher = useFetcher<ActionData>();
  const promoteFetcher = useFetcher<ActionData>();
  const imagesFetcher = useFetcher<ImagesLoaderData>();
  const [pinOpen, pinHandlers] = useDisclosure(false);
  const [promoteOpen, promoteHandlers] = useDisclosure(false);
  const [image, setImage] = useState("");
  const [pinSync, setPinSync] = useState(true);
  const [promoteSync, setPromoteSync] = useState(true);
  const [theater, setTheater] = useState<TheaterSession | null>(null);

  const stages = status?.stages ?? [];
  const defaultStage = stages[0]?.name ?? "";
  const [pinStage, setPinStage] = useState("");
  const stageValue = pinStage || defaultStage;
  const fromStage = stages[0];
  const toStage = stages[1];

  const theaterLive = theater != null && (theater.result == null || theater.sync);
  useEffect(() => {
    const every = theaterLive ? 2000 : 4000;
    const id = window.setInterval(() => {
      if (revalidator.state === "idle") void revalidator.revalidate();
    }, every);
    return () => window.clearInterval(id);
  }, [revalidator, theaterLive]);

  useEffect(() => {
    if (!pinOpen || !status?.name) return;
    void imagesFetcher.load(`/deployables/${encodeURIComponent(status.name)}/images`);
    // Fetcher identity changes; reload only when the modal opens for a deployable.
    // eslint-disable-next-line react-hooks/exhaustive-deps -- load on open
  }, [pinOpen, status?.name]);

  const imageOptions = imagesFetcher.data?.images ?? [];
  const imagesError = imagesFetcher.data?.error ?? null;
  const imagesLoading =
    pinOpen && (imagesFetcher.state !== "idle" || imagesFetcher.data == null);
  const imagesSource = imagesFetcher.data?.source ?? "";
  const selectedImage = image || imageOptions[0]?.ref || "";

  useFetcherResult(pinFetcher, (data) => {
    setTheater((t) => {
      if (t?.kind !== "pin") return t;
      return data.ok
        ? {
            ...t,
            result: data.result,
            resultAt: t.resultAt ?? Date.now(),
            error: undefined,
          }
        : { ...t, error: data.error };
    });
    if (!data.ok) {
      notifyActionError("Pin failed", data.error);
      return;
    }
    if (data.result?.dryRun) {
      notifyActionSuccess(
        "Pin",
        `Wrote overlay${mutationNote(data.result, { argoAvailable: status?.sync })}`,
      );
      pinHandlers.close();
    }
    setImage("");
    void revalidator.revalidate();
  });

  useFetcherResult(promoteFetcher, (data) => {
    setTheater((t) => {
      if (t?.kind !== "promote") return t;
      return data.ok
        ? {
            ...t,
            result: data.result,
            resultAt: t.resultAt ?? Date.now(),
            error: undefined,
          }
        : { ...t, error: data.error };
    });
    if (!data.ok) {
      notifyActionError("Promote failed", data.error);
      return;
    }
    if (data.result?.dryRun) {
      notifyActionSuccess(
        "Promote",
        `Copied pin${mutationNote(data.result, { argoAvailable: status?.sync })}`,
      );
      promoteHandlers.close();
    }
    void revalidator.revalidate();
  });

  const previewDiff = useMemo(() => {
    if (pinFetcher.data?.ok) return pinFetcher.data.result?.diff ?? "";
    if (promoteFetcher.data?.ok) return promoteFetcher.data.result?.diff ?? "";
    return "";
  }, [pinFetcher.data, promoteFetcher.data]);

  if (error != null || status == null) {
    return (
      <Stack gap="lg">
        <PageHeader title="Deployable" crumbs={[deployablesCrumb]} />
        <Alert color="red" title="Could not load">
          {error ?? "unknown error"}
        </Alert>
      </Stack>
    );
  }

  return (
    <Stack gap="lg">
      <PageHeader
        title={status.name}
        crumbs={[deployablesCrumb]}
        description={
          <Stack gap={6}>
            <Text size="sm" c="dimmed">
              {status.namespace} · {status.imageRepo}
            </Text>
            {status.update != null ? <RegistryUpdateHint update={status.update} /> : null}
            <DeployableLinkLabels
              repoURL={status.repoURL}
              projectURL={status.projectURL}
            />
          </Stack>
        }
        actions={
          <Group gap="sm">
            <Button
              variant="default"
              onClick={() => {
                setPinSync(true);
                pinHandlers.open();
              }}
              disabled={stages.length === 0}
            >
              Pin image
            </Button>
            <Button
              onClick={() => {
                setPromoteSync(true);
                promoteHandlers.open();
              }}
              disabled={fromStage == null || toStage == null}
            >
              Promote
            </Button>
          </Group>
        }
      />

      <MutationModeAlert apply={status.apply} push={status.push} />

      {theater ? (
        <ReleaseTheater
          session={theater}
          submitting={
            theater.kind === "pin"
              ? pinFetcher.state !== "idle"
              : promoteFetcher.state !== "idle"
          }
          stage={stages.find((st) => st.name === theater.stage)}
          onDismiss={() => setTheater(null)}
        />
      ) : null}

      <ReleaseFlow stages={stages} flow={status.flow} />

      <ResourceTable
        headers={[
          "Stage",
          "Hostname",
          "Image",
          "Sync",
          "Health",
          "Ready",
          "Deployed",
          "Links",
          "",
        ]}
        isEmpty={stages.length === 0}
        minWidth={880}
      >
        {stages.map((st) => (
          <Table.Tr key={st.name}>
            <Table.Td className="db-cell-fit" fw={600}>
              {st.name}
            </Table.Td>
            <Table.Td className="db-cell-fit">
              <HostnameLink hostname={st.hostname} />
            </Table.Td>
            <Table.Td className="db-cell-clip">
              <CompactImage value={st.image} empty="—" />
            </Table.Td>
            <Table.Td className="db-cell-fit">
              <StatusBadge status={st.sync} href={st.argoURL} />
            </Table.Td>
            <Table.Td>
              <StatusBadge status={st.health} href={st.argoURL} />
              {st.message ? (
                <Text size="xs" c="dimmed">
                  {st.message}
                </Text>
              ) : null}
            </Table.Td>
            <Table.Td className="db-cell-fit">
              <StageReady workload={st.workload} />
            </Table.Td>
            <Table.Td className="db-cell-fit">
              <RelativeTime value={st.deployedAt} />
            </Table.Td>
            <Table.Td className="db-cell-fit">
              <StageObservabilityIcons
                headlampURL={st.headlampURL}
                grafanaURL={st.grafanaURL}
                logsURL={st.logsURL}
              />
            </Table.Td>
            <Table.Td className="db-cell-fit">
              <Button
                component={Link}
                to={`/deployables/${status.name}/reconcile/${st.name}`}
                variant="default"
                size="compact-sm"
              >
                Reconcile
              </Button>
            </Table.Td>
          </Table.Tr>
        ))}
      </ResourceTable>

      <WorkloadPods stages={stages} />

      <WorkflowRuns workflows={status.workflows} />

      {previewDiff && theater == null ? (
        <DiffPanel diff={previewDiff} title="Last mutation diff" />
      ) : null}

      <ReleaseHistory
        stages={stages.map((st) => st.name)}
        releases={history?.releases ?? []}
        error={historyError}
      />

      <ConfirmActionModal
        opened={pinOpen}
        onClose={() => {
          pinHandlers.close();
          setImage("");
        }}
        loading={pinFetcher.state !== "idle"}
        title={`Pin ${status.name}`}
        confirmLabel={mutationCommitLabel(status, "pin", status.sync && pinSync)}
        confirmDisabled={!selectedImage.trim() || imagesLoading}
        argoURL={stages.find((s) => s.name === stageValue)?.argoURL}
        message={
          <Stack gap="sm">
            <Select
              label="Stage"
              data={stages.map((s) => s.name)}
              value={stageValue}
              onChange={(v) => setPinStage(v ?? "")}
            />
            {imagesLoading ? (
              <Select
                label="Image"
                data={[]}
                placeholder="Loading published images…"
                disabled
              />
            ) : imageOptions.length > 0 ? (
              <Select
                label="Image"
                searchable
                allowDeselect={false}
                data={imageOptions.map((img) => ({
                  value: img.ref,
                  label: img.tag || shortDigest(img.digest) || img.ref,
                }))}
                value={selectedImage || null}
                onChange={(v) => setImage(v ?? "")}
                nothingFoundMessage="No matching images"
                maxDropdownHeight={320}
                renderOption={({ option }) => {
                  const img = imageOptions.find((i) => i.ref === option.value);
                  return <ImageOption img={img} label={option.label} />;
                }}
              />
            ) : (
              <TextInput
                label="Image"
                placeholder="repository:tag@sha256:…"
                value={selectedImage}
                onChange={(e) => setImage(e.currentTarget.value)}
              />
            )}
            <Text size="xs" c="dimmed">
              {pinImagesHint({
                loading: imagesLoading,
                error: imagesError,
                count: imageOptions.length,
                source: imagesSource,
              })}
            </Text>
            <ArgoSyncCheckbox
              show={status.apply && status.sync}
              checked={pinSync}
              onChange={setPinSync}
              stage={stageValue}
            />
            <MutationGitHint
              apply={status.apply}
              push={status.push}
              sync={status.sync && pinSync}
              syncStage={stageValue}
            />
          </Stack>
        }
        onConfirm={() => {
          const sync = status.sync && pinSync;
          if (status.apply) {
            setTheater({
              kind: "pin",
              stage: stageValue,
              image: selectedImage,
              startedAt: Date.now(),
              apply: status.apply,
              push: status.push,
              sync,
              initialPodNames: podNamesFor(stages, stageValue),
            });
            pinHandlers.close();
          }
          void pinFetcher.submit(
            {
              intent: "pin",
              stage: stageValue,
              image: selectedImage,
              ...(status.sync ? { sync: pinSync ? "true" : "false" } : {}),
              ...(status.apply ? { wait: "false" } : {}),
            },
            { method: "post" },
          );
        }}
      />

      <ConfirmActionModal
        opened={promoteOpen}
        onClose={promoteHandlers.close}
        loading={promoteFetcher.state !== "idle"}
        title={`Promote ${status.name}`}
        confirmLabel={mutationCommitLabel(status, "promote", status.sync && promoteSync)}
        argoURL={toStage?.argoURL}
        message={
          fromStage && toStage ? (
            <Stack gap="sm">
              <Text size="sm">
                Copy the pinned image from <strong>{fromStage.name}</strong> (
                <CompactImage value={fromStage.image} empty="unpinned" />) to{" "}
                <strong>{toStage.name}</strong>. Homelab must be healthy when Argo is
                configured.
              </Text>
              <ArgoSyncCheckbox
                show={status.apply && status.sync}
                checked={promoteSync}
                onChange={setPromoteSync}
                stage={toStage.name}
              />
              <MutationGitHint
                apply={status.apply}
                push={status.push}
                sync={status.sync && promoteSync}
                syncStage={toStage.name}
              />
            </Stack>
          ) : (
            <Text size="sm">Need at least two stages to promote.</Text>
          )
        }
        onConfirm={() => {
          if (!fromStage || !toStage) return;
          const hop = status.flow?.hops.find(
            (h) => h.from === fromStage.name && h.to === toStage.name,
          );
          const sync = status.sync && promoteSync;
          if (status.apply) {
            setTheater({
              kind: "promote",
              stage: toStage.name,
              image: hop?.sourceImage || fromStage.image || "",
              startedAt: Date.now(),
              apply: status.apply,
              push: status.push,
              sync,
              initialPodNames: podNamesFor(stages, toStage.name),
            });
            promoteHandlers.close();
          }
          void promoteFetcher.submit(
            {
              intent: "promote",
              from: fromStage.name,
              to: toStage.name,
              ...(hop?.sourceImage ? { image: hop.sourceImage } : {}),
              ...(status.sync ? { sync: promoteSync ? "true" : "false" } : {}),
              ...(status.apply ? { wait: "false" } : {}),
            },
            { method: "post" },
          );
        }}
      />
    </Stack>
  );
}

function podNamesFor(
  stages: { name: string; workload?: { pods?: { name: string }[] } }[],
  stage: string,
): string[] {
  return (stages.find((st) => st.name === stage)?.workload?.pods ?? []).map(
    (p) => p.name,
  );
}

function pinImagesHint({
  loading,
  error,
  count,
  source,
}: {
  loading: boolean;
  error: string | null;
  count: number;
  source: string;
}): string {
  if (loading) return "Loading published images…";
  if (error != null) return `Could not list images (${error}). Paste a ref instead.`;
  if (count === 0) return "No tagged versions. Paste a ref instead.";
  switch (source) {
    case "commits":
      return "Newest git commits first (`main-<sha>` tags). Token needs read:packages for GHCR digests.";
    case "dockerhub":
      return "Newest Docker Hub tags first.";
    default:
      return "Newest GHCR versions first.";
  }
}

function MutationModeAlert({ apply, push }: { apply: boolean; push: boolean }) {
  if (!apply) {
    return (
      <Alert color="yellow" title="Dry-run">
        Mutations preview a git diff and do not commit. Start the API with{" "}
        <Text span ff="monospace">
          --apply
        </Text>{" "}
        and{" "}
        <Text span ff="monospace">
          DEPLOYBOT_OPS_REPO
        </Text>{" "}
        to write local commits, plus{" "}
        <Text span ff="monospace">
          --push
        </Text>{" "}
        to update the remote.
      </Alert>
    );
  }
  if (!push) {
    return (
      <Alert color="yellow" title="Local commits only">
        Mutations commit locally and do not push. Start the API with{" "}
        <Text span ff="monospace">
          --push
        </Text>{" "}
        to update the ops remote. Never force-pushes.
      </Alert>
    );
  }
  return null;
}

function ImageOption({ img, label }: { img?: ImageVersion; label: string }) {
  return (
    <Group justify="space-between" gap="sm" wrap="nowrap" w="100%">
      <Stack gap={0}>
        <Text size="sm">{img?.tag || label}</Text>
        {img?.digest ? (
          <Text size="xs" c="dimmed" ff="monospace">
            {shortDigest(img.digest)}
          </Text>
        ) : null}
      </Stack>
      {img?.createdAt ? (
        <Text size="xs" c="dimmed">
          {formatAbsolute(img.createdAt)}
        </Text>
      ) : null}
    </Group>
  );
}
