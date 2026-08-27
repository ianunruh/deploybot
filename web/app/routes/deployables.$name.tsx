import {
  Alert,
  Badge,
  Button,
  Group,
  Stack,
  Tabs,
  Text,
  TextInput,
  Select,
} from "@mantine/core";
import { useDisclosure } from "@mantine/hooks";
import { useEffect, useMemo, useState } from "react";
import {
  Outlet,
  useFetcher,
  useLocation,
  useNavigate,
  useRevalidator,
} from "react-router";

import type { Route } from "./+types/deployables.$name";
import type { ChangelogLoaderData } from "./deployables.$name.changelog";
import type { ImagesLoaderData } from "./deployables.$name.images";
import type { WorkloadsLoaderData } from "./deployables.$name.workloads";
import {
  getDeployable,
  actorHeaders,
  pinDeployable,
  promoteDeployable,
  rollbackDeployable,
  type DeployableStatus,
  type ImageVersion,
  type MutationResult,
  type StageStatus,
  type UpdateStatus as RegistryUpdate,
} from "~/lib/api.server";
import { notifyActionError, notifyActionSuccess } from "~/lib/action-feedback";
import { formatAbsolute } from "~/lib/time";
import { useFetcherResult } from "~/lib/use-fetcher-result";
import { CompactImage, shortDigest } from "~/ui/compact-image";
import { ConfirmActionModal } from "~/ui/confirm-action-modal";
import { DiffPanel } from "~/ui/diff-panel";
import { ReleaseTheater, type TheaterSession } from "~/ui/release-theater";
import { DeployableLinkLabels } from "~/ui/external-links";
import {
  ArgoSyncCheckbox,
  formFlag,
  MutationGitHint,
  mutationCommitLabel,
  mutationNote,
} from "~/ui/mutation-controls";
import { PageHeader } from "~/ui/page-header";
import { PromoteChangelog } from "~/ui/promote-changelog";
import { updatesNameHref } from "~/ui/resource-filter";
import { UpdateBadge } from "~/ui/status-badge";

export type DeployableContext = {
  status: DeployableStatus;
  stages: StageStatus[];
  onRollback: (stage: string, image: string) => void;
};

export function meta({ params }: Route.MetaArgs) {
  return [{ title: `${params.name} · deploybot` }];
}

export async function loader({ params }: Route.LoaderArgs) {
  const name = params.name;
  if (!name) throw new Response("Missing name", { status: 400 });
  try {
    return {
      status: await getDeployable(name),
      error: null as string | null,
    };
  } catch (err) {
    return {
      status: null,
      error: err instanceof Error ? err.message : String(err),
    };
  }
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
            {
              sync: formFlag(form, "sync"),
              wait: formFlag(form, "wait"),
              headers: actorHeaders(request),
            },
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
              headers: actorHeaders(request),
            },
          ),
        } satisfies ActionData;
      case "rollback":
        return {
          ok: true,
          intent,
          result: await rollbackDeployable(
            name,
            String(form.get("stage") ?? ""),
            String(form.get("image") ?? ""),
            {
              sync: formFlag(form, "sync"),
              wait: formFlag(form, "wait"),
              headers: actorHeaders(request),
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

const deployablesCrumb = { label: "Catalog", to: "/" };

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
      <UpdateBadge
        stale={update.stale}
        to={update.stale ? updatesNameHref(update.name) : undefined}
      />
      <Text size="sm" c="dimmed">
        {body}
      </Text>
    </Group>
  );
}

function tabFromPath(pathname: string): string {
  if (pathname.endsWith("/workflows")) return "workflows";
  if (pathname.endsWith("/history")) return "history";
  if (pathname.endsWith("/workload")) return "workload";
  return "overview";
}

function tabPath(name: string, tab: string | null): string {
  const base = `/deployables/${name}`;
  if (tab === "workflows") return `${base}/workflows`;
  if (tab === "history") return `${base}/history`;
  if (tab === "workload") return `${base}/workload`;
  return base;
}

export default function DeployableDetail({ loaderData }: Route.ComponentProps) {
  const { status, error } = loaderData;
  const revalidator = useRevalidator();
  const navigate = useNavigate();
  const location = useLocation();
  const pinFetcher = useFetcher<ActionData>();
  const promoteFetcher = useFetcher<ActionData>();
  const rollbackFetcher = useFetcher<ActionData>();
  const imagesFetcher = useFetcher<ImagesLoaderData>();
  const changelogFetcher = useFetcher<ChangelogLoaderData>();
  const workloadsFetcher = useFetcher<WorkloadsLoaderData>();
  const [pinOpen, pinHandlers] = useDisclosure(false);
  const [promoteOpen, promoteHandlers] = useDisclosure(false);
  const [rollbackOpen, rollbackHandlers] = useDisclosure(false);
  const [image, setImage] = useState("");
  const [pinSync, setPinSync] = useState(true);
  const [promoteSync, setPromoteSync] = useState(true);
  const [rollbackSync, setRollbackSync] = useState(true);
  const [rollbackTarget, setRollbackTarget] = useState<{
    stage: string;
    image: string;
  } | null>(null);
  const [theater, setTheater] = useState<TheaterSession | null>(null);

  const stages = useMemo(
    () => mergeWorkloads(status?.stages ?? [], workloadsFetcher.data?.stages),
    [status?.stages, workloadsFetcher.data?.stages],
  );
  const defaultStage = stages[0]?.name ?? "";
  const [pinStage, setPinStage] = useState("");
  const stageValue = pinStage || defaultStage;
  const fromStage = stages[0];
  const toStage = stages[1];
  const tab = tabFromPath(location.pathname);
  const showWorkflows = Boolean(status?.source);

  const theaterLive = theater != null && (theater.result == null || theater.sync);
  useEffect(() => {
    if (!status?.name) return;
    const workloadsPath = `/deployables/${encodeURIComponent(status.name)}/workloads`;
    const needWorkloads = tab === "overview" || tab === "workload" || theaterLive;
    if (needWorkloads) void workloadsFetcher.load(workloadsPath);
    const every = theaterLive ? 2000 : 4000;
    const id = window.setInterval(() => {
      if (revalidator.state === "idle") void revalidator.revalidate();
      if (needWorkloads) void workloadsFetcher.load(workloadsPath);
    }, every);
    return () => window.clearInterval(id);
    // Fetcher identity changes; poll while this deployable is open.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [revalidator, theaterLive, status?.name, tab]);

  useEffect(() => {
    if (!status?.name || showWorkflows || tab !== "workflows") return;
    void navigate(tabPath(status.name, "overview"), { replace: true });
  }, [navigate, showWorkflows, status?.name, tab]);

  useEffect(() => {
    if (!pinOpen || !status?.name) return;
    void imagesFetcher.load(`/deployables/${encodeURIComponent(status.name)}/images`);
    // Fetcher identity changes; reload only when the modal opens for a deployable.
    // eslint-disable-next-line react-hooks/exhaustive-deps -- load on open
  }, [pinOpen, status?.name]);

  useEffect(() => {
    if (!promoteOpen || !status?.name || !fromStage?.name || !toStage?.name) return;
    const q = new URLSearchParams({ from: fromStage.name, to: toStage.name });
    void changelogFetcher.load(
      `/deployables/${encodeURIComponent(status.name)}/changelog?${q}`,
    );
    // Fetcher identity changes; reload only when the modal opens for a hop.
    // eslint-disable-next-line react-hooks/exhaustive-deps -- load on open
  }, [promoteOpen, status?.name, fromStage?.name, toStage?.name]);

  const imageOptions = imagesFetcher.data?.images ?? [];
  const imagesError = imagesFetcher.data?.error ?? null;
  const imagesLoading =
    pinOpen && (imagesFetcher.state !== "idle" || imagesFetcher.data == null);
  const imagesSource = imagesFetcher.data?.source ?? "";
  const selectedImage = image || imageOptions[0]?.ref || "";
  const actionPath = status?.name ? `/deployables/${status.name}` : ".";

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

  useFetcherResult(rollbackFetcher, (data) => {
    setTheater((t) => {
      if (t?.kind !== "rollback") return t;
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
      notifyActionError("Rollback failed", data.error);
      return;
    }
    if (data.result?.dryRun) {
      notifyActionSuccess(
        "Rollback",
        `Wrote overlay${mutationNote(data.result, { argoAvailable: status?.sync })}`,
      );
      rollbackHandlers.close();
    }
    void revalidator.revalidate();
  });

  const previewDiff = useMemo(() => {
    if (pinFetcher.data?.ok) return pinFetcher.data.result?.diff ?? "";
    if (promoteFetcher.data?.ok) return promoteFetcher.data.result?.diff ?? "";
    if (rollbackFetcher.data?.ok) return rollbackFetcher.data.result?.diff ?? "";
    return "";
  }, [pinFetcher.data, promoteFetcher.data, rollbackFetcher.data]);

  function openRollback(stage: string, imageRef: string) {
    setRollbackTarget({ stage, image: imageRef });
    setRollbackSync(true);
    rollbackHandlers.open();
  }

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
            {status.summary ? <Text size="sm">{status.summary}</Text> : null}
            <Group gap="xs" wrap="wrap">
              {status.project ? (
                <Badge variant="light" size="sm" tt="uppercase" radius="sm">
                  {status.project}
                </Badge>
              ) : null}
              <Text size="sm" c="dimmed">
                {status.namespace} · {status.imageRepo}
              </Text>
            </Group>
            {status.update != null ? <RegistryUpdateHint update={status.update} /> : null}
            <DeployableLinkLabels
              repoURL={status.repoURL}
              projectURL={status.projectURL}
              docsURL={status.docsURL}
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

      {stages
        .filter(
          (st) =>
            /^(degraded|missing)$/i.test(st.health) &&
            st.previousRef &&
            theater?.stage !== st.name,
        )
        .map((st) => (
          <Alert key={st.name} color="red" title={`${st.name} is ${st.health}`}>
            <Group justify="space-between" align="center" gap="sm" wrap="wrap">
              <Text size="sm">
                Last pin on this stage was{" "}
                <CompactImage value={st.previousImage} empty="the previous digest" />.
                {st.message ? ` ${st.message}` : ""}
              </Text>
              <Button
                size="compact-sm"
                color="red"
                onClick={() => openRollback(st.name, st.previousRef ?? "")}
              >
                Rollback
              </Button>
            </Group>
          </Alert>
        ))}

      {theater ? (
        <ReleaseTheater
          session={theater}
          submitting={
            theater.kind === "pin"
              ? pinFetcher.state !== "idle"
              : theater.kind === "rollback"
                ? rollbackFetcher.state !== "idle"
                : promoteFetcher.state !== "idle"
          }
          stage={stages.find((st) => st.name === theater.stage)}
          onDismiss={() => setTheater(null)}
        />
      ) : null}

      {previewDiff && theater == null ? (
        <DiffPanel diff={previewDiff} title="Last mutation diff" />
      ) : null}

      <Stack gap="md">
        <Tabs
          value={tab}
          onChange={(value) => {
            void navigate(tabPath(status.name, value));
          }}
        >
          <Tabs.List>
            <Tabs.Tab value="overview">Overview</Tabs.Tab>
            <Tabs.Tab value="history">History</Tabs.Tab>
            <Tabs.Tab value="workload">Workload</Tabs.Tab>
            {showWorkflows ? <Tabs.Tab value="workflows">CI Runs</Tabs.Tab> : null}
          </Tabs.List>
        </Tabs>

        <Outlet
          context={
            { status, stages, onRollback: openRollback } satisfies DeployableContext
          }
        />
      </Stack>

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
            { method: "post", action: actionPath },
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
                <strong>{toStage.name}</strong> (
                <CompactImage value={toStage.image} empty="unpinned" />
                ). Homelab must be healthy when Argo is configured.
              </Text>
              <PromoteChangelog
                changelog={changelogFetcher.data?.changelog}
                error={changelogFetcher.data?.error}
                loading={
                  promoteOpen &&
                  (changelogFetcher.state !== "idle" || changelogFetcher.data == null)
                }
                to={toStage.name}
              />
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
            { method: "post", action: actionPath },
          );
        }}
      />

      <ConfirmActionModal
        opened={rollbackOpen}
        onClose={() => {
          rollbackHandlers.close();
          setRollbackTarget(null);
        }}
        loading={rollbackFetcher.state !== "idle"}
        title={`Rollback ${status.name}`}
        confirmLabel={mutationCommitLabel(
          status,
          "rollback",
          status.sync && rollbackSync,
        )}
        confirmDisabled={!rollbackTarget?.image.trim()}
        argoURL={stages.find((s) => s.name === rollbackTarget?.stage)?.argoURL}
        message={
          rollbackTarget ? (
            <Stack gap="sm">
              <Text size="sm">
                Re-pin <strong>{rollbackTarget.stage}</strong> to{" "}
                <CompactImage value={rollbackTarget.image} empty={rollbackTarget.image} />
                . The overlay change is the same as pin. Git remains the source of truth.
              </Text>
              <ArgoSyncCheckbox
                show={status.apply && status.sync}
                checked={rollbackSync}
                onChange={setRollbackSync}
                stage={rollbackTarget.stage}
              />
              <MutationGitHint
                apply={status.apply}
                push={status.push}
                sync={status.sync && rollbackSync}
                syncStage={rollbackTarget.stage}
              />
            </Stack>
          ) : (
            <Text size="sm">Pick a previous digest to roll back to.</Text>
          )
        }
        onConfirm={() => {
          if (!rollbackTarget) return;
          const sync = status.sync && rollbackSync;
          if (status.apply) {
            setTheater({
              kind: "rollback",
              stage: rollbackTarget.stage,
              image: rollbackTarget.image,
              startedAt: Date.now(),
              apply: status.apply,
              push: status.push,
              sync,
              initialPodNames: podNamesFor(stages, rollbackTarget.stage),
            });
            rollbackHandlers.close();
          }
          void rollbackFetcher.submit(
            {
              intent: "rollback",
              stage: rollbackTarget.stage,
              image: rollbackTarget.image,
              ...(status.sync ? { sync: rollbackSync ? "true" : "false" } : {}),
              ...(status.apply ? { wait: "false" } : {}),
            },
            { method: "post", action: actionPath },
          );
        }}
      />
    </Stack>
  );
}

function mergeWorkloads(
  stages: StageStatus[],
  live?: Array<{ name: string; workload?: StageStatus["workload"] }>,
): StageStatus[] {
  if (live == null) return stages;
  const byName = new Map(live.map((s) => [s.name, s.workload]));
  return stages.map((st) =>
    byName.has(st.name) ? { ...st, workload: byName.get(st.name) } : st,
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
