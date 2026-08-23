import { Alert, Button, Group, Stack, Text, TextInput, Select } from "@mantine/core";
import { useDisclosure } from "@mantine/hooks";
import { useEffect, useMemo, useState } from "react";
import { Link, useFetcher, useRevalidator } from "react-router";

import type { Route } from "./+types/deployables.$name";
import {
  getDeployable,
  pinDeployable,
  promoteDeployable,
  type MutationResult,
} from "~/lib/api.server";
import { notifyActionError, notifyActionSuccess } from "~/lib/action-feedback";
import { useFetcherResult } from "~/lib/use-fetcher-result";
import { ConfirmActionModal } from "~/ui/confirm-action-modal";
import { DiffPanel } from "~/ui/diff-panel";
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

export default function DeployableDetail({ loaderData }: Route.ComponentProps) {
  const { status, error } = loaderData;
  const revalidator = useRevalidator();
  const pinFetcher = useFetcher<ActionData>();
  const promoteFetcher = useFetcher<ActionData>();
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

  useFetcherResult(pinFetcher, (data) => {
    if (!data.ok) {
      notifyActionError("Pin failed", data.error);
      return;
    }
    const extra = data.result?.dryRun ? " (dry-run)" : "";
    notifyActionSuccess("Pin", `Wrote overlay${extra}`);
    pinHandlers.close();
    void revalidator.revalidate();
  });

  useFetcherResult(promoteFetcher, (data) => {
    if (!data.ok) {
      notifyActionError("Promote failed", data.error);
      return;
    }
    const extra = data.result?.dryRun ? " (dry-run)" : "";
    notifyActionSuccess("Promote", `Copied pin${extra}`);
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
        <PageHeader title="Deployable" />
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
        description={`${status.namespace} · ${status.imageRepo}`}
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

      {!status.apply && (
        <Alert color="yellow" title="Dry-run">
          Mutations preview a git diff and do not commit. Start the API with{" "}
          <Text span ff="monospace">
            --apply
          </Text>{" "}
          and{" "}
          <Text span ff="monospace">
            DEPLOYBOT_OPS_REPO
          </Text>{" "}
          to write local commits.
        </Alert>
      )}

      <ResourceTable
        headers={["Stage", "Hostname", "Image", "Sync", "Health", ""]}
        isEmpty={stages.length === 0}
        minWidth={720}
      >
        {stages.map((st) => (
          <Table.Tr key={st.name}>
            <Table.Td fw={600}>{st.name}</Table.Td>
            <Table.Td>
              <Text size="sm" ff="monospace">
                {st.hostname}
              </Text>
            </Table.Td>
            <Table.Td>
              <Text size="xs" ff="monospace" lineClamp={2}>
                {st.image || "—"}
              </Text>
            </Table.Td>
            <Table.Td>
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
            <Table.Td>
              <Button
                component={Link}
                to={`/deployables/${status.name}/sync/${st.name}`}
                variant="default"
                size="compact-sm"
              >
                Sync
              </Button>
            </Table.Td>
          </Table.Tr>
        ))}
      </ResourceTable>

      {previewDiff ? <DiffPanel diff={previewDiff} title="Last mutation diff" /> : null}

      <ConfirmActionModal
        opened={pinOpen}
        onClose={pinHandlers.close}
        loading={pinFetcher.state !== "idle"}
        title={`Pin ${status.name}`}
        confirmLabel={status.apply ? "Commit pin" : "Preview pin"}
        message={
          <Stack gap="sm">
            <Select
              label="Stage"
              data={stages.map((s) => s.name)}
              value={stageValue}
              onChange={(v) => setPinStage(v ?? "")}
            />
            <TextInput
              label="Image"
              placeholder="ghcr.io/ianunruh/kmc@sha256:…"
              value={image}
              onChange={(e) => setImage(e.currentTarget.value)}
            />
            <Text size="xs" c="dimmed">
              Actions already publish main-&lt;sha&gt; and digest tags.
            </Text>
          </Stack>
        }
        onConfirm={() => {
          void pinFetcher.submit(
            { intent: "pin", stage: stageValue, image },
            { method: "post" },
          );
        }}
      />

      <ConfirmActionModal
        opened={promoteOpen}
        onClose={promoteHandlers.close}
        loading={promoteFetcher.state !== "idle"}
        title={`Promote ${status.name}`}
        confirmLabel={status.apply ? "Commit promote" : "Preview promote"}
        message={
          fromStage && toStage ? (
            <Text size="sm">
              Copy the pinned image from <strong>{fromStage.name}</strong> (
              {fromStage.image || "unpinned"}) to <strong>{toStage.name}</strong>. Homelab
              must be healthy when Argo is configured.
            </Text>
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
