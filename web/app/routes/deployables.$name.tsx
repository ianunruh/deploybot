import { Alert, Button, Group, Stack, Text, TextInput, Select } from "@mantine/core";
import { useDisclosure } from "@mantine/hooks";
import { useEffect, useMemo, useState } from "react";
import { Link, useFetcher, useRevalidator } from "react-router";

import type { Route } from "./+types/deployables.$name";
import type { ImagesLoaderData } from "./deployables.$name.images";
import {
  getDeployable,
  pinDeployable,
  promoteDeployable,
  type ImageVersion,
  type MutationResult,
} from "~/lib/api.server";
import { notifyActionError, notifyActionSuccess } from "~/lib/action-feedback";
import { useFetcherResult } from "~/lib/use-fetcher-result";
import { ConfirmActionModal } from "~/ui/confirm-action-modal";
import { DiffPanel } from "~/ui/diff-panel";
import { ArgoCDLink, DeployableLinkLabels, HostnameLink } from "~/ui/external-links";
import { PageHeader } from "~/ui/page-header";
import { ResourceTable, Table } from "~/ui/resource-table";
import { StatusBadge } from "~/ui/status-badge";

export function meta({ params }: Route.MetaArgs) {
  return [{ title: `${params.name} · deploybot` }];
}

export async function loader({ params }: Route.LoaderArgs) {
  const name = params.name;
  if (!name) throw new Response("Missing name", { status: 400 });
  try {
    const status = await getDeployable(name);
    return { status, error: null as string | null };
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

export default function DeployableDetail({ loaderData }: Route.ComponentProps) {
  const { status, error } = loaderData;
  const revalidator = useRevalidator();
  const pinFetcher = useFetcher<ActionData>();
  const promoteFetcher = useFetcher<ActionData>();
  const imagesFetcher = useFetcher<ImagesLoaderData>();
  const [pinOpen, pinHandlers] = useDisclosure(false);
  const [promoteOpen, promoteHandlers] = useDisclosure(false);
  const [image, setImage] = useState("");

  const stages = status?.stages ?? [];
  const defaultStage = stages[0]?.name ?? "";
  const [pinStage, setPinStage] = useState("");
  const stageValue = pinStage || defaultStage;
  const fromStage = stages[0];
  const toStage = stages[1];

  useEffect(() => {
    const id = window.setInterval(() => {
      if (revalidator.state === "idle") void revalidator.revalidate();
    }, 4000);
    return () => window.clearInterval(id);
  }, [revalidator]);

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
    if (!data.ok) {
      notifyActionError("Pin failed", data.error);
      return;
    }
    notifyActionSuccess("Pin", `Wrote overlay${mutationNote(data.result)}`);
    pinHandlers.close();
    setImage("");
    void revalidator.revalidate();
  });

  useFetcherResult(promoteFetcher, (data) => {
    if (!data.ok) {
      notifyActionError("Promote failed", data.error);
      return;
    }
    notifyActionSuccess("Promote", `Copied pin${mutationNote(data.result)}`);
    promoteHandlers.close();
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
              onClick={() => pinHandlers.open()}
              disabled={stages.length === 0}
            >
              Pin image
            </Button>
            <Button
              onClick={() => promoteHandlers.open()}
              disabled={fromStage == null || toStage == null}
            >
              Promote
            </Button>
          </Group>
        }
      />

      <MutationModeAlert apply={status.apply} push={status.push} />

      <ResourceTable
        headers={["Stage", "Hostname", "Image", "Sync", "Health", "", ""]}
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
              <StatusBadge status={st.sync} />
            </Table.Td>
            <Table.Td>
              <StatusBadge status={st.health} />
              {st.message ? (
                <Text size="xs" c="dimmed">
                  {st.message}
                </Text>
              ) : null}
            </Table.Td>
            <Table.Td className="db-cell-fit">
              <Button
                component={Link}
                to={`/deployables/${status.name}/sync/${st.name}`}
                variant="default"
                size="compact-sm"
              >
                Sync
              </Button>
            </Table.Td>
            <Table.Td className="db-cell-fit">
              {st.argoURL ? (
                <ArgoCDLink href={st.argoURL} />
              ) : (
                <Text size="sm" c="dimmed">
                  —
                </Text>
              )}
            </Table.Td>
          </Table.Tr>
        ))}
      </ResourceTable>

      {previewDiff ? <DiffPanel diff={previewDiff} title="Last mutation diff" /> : null}

      <ConfirmActionModal
        opened={pinOpen}
        onClose={() => {
          pinHandlers.close();
          setImage("");
        }}
        loading={pinFetcher.state !== "idle"}
        title={`Pin ${status.name}`}
        confirmLabel={commitLabel(status, "pin")}
        confirmDisabled={!selectedImage.trim() || imagesLoading}
        size="lg"
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
                placeholder="ghcr.io/ianunruh/kmc@sha256:…"
                value={selectedImage}
                onChange={(e) => setImage(e.currentTarget.value)}
              />
            )}
            {imagesLoading ? (
              <Text size="xs" c="dimmed">
                Loading published images…
              </Text>
            ) : imagesError != null ? (
              <Text size="xs" c="dimmed">
                Could not list images ({imagesError}). Paste a ref instead.
              </Text>
            ) : imageOptions.length === 0 ? (
              <Text size="xs" c="dimmed">
                No tagged GHCR versions. Paste a ref instead.
              </Text>
            ) : imagesSource === "commits" ? (
              <Text size="xs" c="dimmed">
                Newest git commits first (`main-&lt;sha&gt;` tags). Token needs
                read:packages for GHCR digests.
              </Text>
            ) : (
              <Text size="xs" c="dimmed">
                Newest GHCR versions first.
              </Text>
            )}
            <MutationGitHint apply={status.apply} push={status.push} />
          </Stack>
        }
        onConfirm={() => {
          void pinFetcher.submit(
            { intent: "pin", stage: stageValue, image: selectedImage },
            { method: "post" },
          );
        }}
      />

      <ConfirmActionModal
        opened={promoteOpen}
        onClose={promoteHandlers.close}
        loading={promoteFetcher.state !== "idle"}
        title={`Promote ${status.name}`}
        confirmLabel={commitLabel(status, "promote")}
        message={
          fromStage && toStage ? (
            <Stack gap="sm">
              <Text size="sm">
                Copy the pinned image from <strong>{fromStage.name}</strong> (
                <CompactImage value={fromStage.image} empty="unpinned" />) to{" "}
                <strong>{toStage.name}</strong>. Homelab must be healthy when Argo is
                configured.
              </Text>
              <MutationGitHint apply={status.apply} push={status.push} />
            </Stack>
          ) : (
            <Text size="sm">Need at least two stages to promote.</Text>
          )
        }
        onConfirm={() => {
          if (!fromStage || !toStage) return;
          void promoteFetcher.submit(
            { intent: "promote", from: fromStage.name, to: toStage.name },
            { method: "post" },
          );
        }}
      />
    </Stack>
  );
}

function commitLabel(status: { apply: boolean; push: boolean }, action: string): string {
  if (!status.apply) return `Preview ${action}`;
  if (status.push) return `Commit and push ${action}`;
  return `Commit ${action}`;
}

function mutationNote(result?: MutationResult): string {
  if (result?.dryRun) return " (dry-run)";
  if (result?.pushed) return " and pushed";
  return "";
}

function MutationGitHint({ apply, push }: { apply: boolean; push: boolean }) {
  let text: string;
  if (!apply) {
    text = "Previews a git diff and does not commit.";
  } else if (!push) {
    text = "Commits locally and does not push.";
  } else {
    text = "Commits and pushes the current branch. Never force-pushes.";
  }
  return (
    <Text size="sm" c="dimmed">
      {text}
    </Text>
  );
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

function CompactImage({ value, empty = "—" }: { value?: string; empty?: string }) {
  if (!value) {
    return (
      <Text size="xs" c="dimmed" span>
        {empty}
      </Text>
    );
  }
  const at = value.indexOf("@");
  const tag = at >= 0 ? value.slice(0, at) : value;
  return (
    <Text className="db-clip-text" size="xs" ff="monospace" title={value}>
      {tag}
    </Text>
  );
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
          {formatWhen(img.createdAt)}
        </Text>
      ) : null}
    </Group>
  );
}

function shortDigest(digest?: string): string {
  if (!digest) return "";
  const hex = digest.replace(/^sha256:/, "");
  return hex ? `sha256:${hex.slice(0, 12)}` : digest;
}

function formatWhen(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return new Intl.DateTimeFormat(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(d);
}
