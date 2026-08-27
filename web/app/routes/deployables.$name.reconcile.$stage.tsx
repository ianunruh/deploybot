import { Alert, Button, Group, Stack, Text } from "@mantine/core";
import { useDisclosure } from "@mantine/hooks";
import { useState } from "react";
import { Link, useFetcher, useRevalidator } from "react-router";

import type { Route } from "./+types/deployables.$name.reconcile.$stage";
import {
  actorHeaders,
  getDeployable,
  previewReconcile,
  reconcileDeployable,
  type MutationResult,
} from "~/lib/api.server";
import { notifyActionError, notifyActionSuccess } from "~/lib/action-feedback";
import { useFetcherResult } from "~/lib/use-fetcher-result";
import { ConfirmActionModal } from "~/ui/confirm-action-modal";
import { DiffPanel } from "~/ui/diff-panel";
import {
  ArgoSyncCheckbox,
  formFlag,
  MutationGitHint,
  mutationCommitLabel,
  mutationNote,
} from "~/ui/mutation-controls";
import { PageHeader } from "~/ui/page-header";

export function meta({ params }: Route.MetaArgs) {
  return [{ title: `Reconcile ${params.stage} · ${params.name} · deploybot` }];
}

export async function loader({ params }: Route.LoaderArgs) {
  const name = params.name;
  const stage = params.stage;
  if (!name || !stage) {
    throw new Response("Missing name or stage", { status: 400 });
  }
  try {
    const [status, preview] = await Promise.all([
      getDeployable(name),
      previewReconcile(name, stage),
    ]);
    return { status, preview, error: null as string | null };
  } catch (err) {
    return {
      status: null,
      preview: null,
      error: err instanceof Error ? err.message : String(err),
    };
  }
}

type ActionData = { ok: true; result: MutationResult } | { ok: false; error: string };

export async function action({ request, params }: Route.ActionArgs) {
  const name = params.name;
  const stage = params.stage;
  if (!name || !stage) {
    return { ok: false, error: "missing name or stage" } satisfies ActionData;
  }
  const form = await request.formData();
  if (String(form.get("intent") ?? "") !== "reconcile") {
    return { ok: false, error: "unknown intent" } satisfies ActionData;
  }
  try {
    return {
      ok: true,
      result: await reconcileDeployable(name, stage, {
        sync: formFlag(form, "sync"),
        wait: formFlag(form, "wait"),
        headers: actorHeaders(request),
      }),
    } satisfies ActionData;
  } catch (err) {
    return {
      ok: false,
      error: err instanceof Error ? err.message : String(err),
    } satisfies ActionData;
  }
}

function reconcileCrumbs(name?: string) {
  const crumbs = [{ label: "Deployables", to: "/" }];
  if (name) {
    crumbs.push({ label: name, to: `/deployables/${name}` });
  }
  return crumbs;
}

export default function ReconcileStage({ loaderData, params }: Route.ComponentProps) {
  const { status, preview, error } = loaderData;
  const revalidator = useRevalidator();
  const fetcher = useFetcher<ActionData>();
  const [reconcileOpen, reconcileHandlers] = useDisclosure(false);
  const [syncArgo, setSyncArgo] = useState(true);

  useFetcherResult(fetcher, (data) => {
    if (!data.ok) {
      notifyActionError("Reconcile failed", data.error);
      return;
    }
    notifyActionSuccess(
      "Reconcile",
      `Wrote manifests${mutationNote(data.result, { argoAvailable: status?.sync })}`,
    );
    reconcileHandlers.close();
    void revalidator.revalidate();
  });

  if (error != null || status == null || preview == null) {
    return (
      <Stack gap="lg">
        <PageHeader title="Reconcile" crumbs={reconcileCrumbs(params.name)} />
        <Alert color="red" title="Could not load">
          {error ?? "unknown error"}
        </Alert>
        <Button
          component={Link}
          to={params.name ? `/deployables/${params.name}` : "/"}
          variant="default"
        >
          Back
        </Button>
      </Stack>
    );
  }

  const files = preview.files ?? [];
  const hasChanges = files.length > 0 && Boolean(preview.diff);
  const committing = fetcher.state !== "idle";
  const stageArgoURL = status.stages.find((s) => s.name === params.stage)?.argoURL;

  return (
    <Stack gap="lg">
      <PageHeader
        title={`Reconcile ${params.stage}`}
        crumbs={reconcileCrumbs(status.name)}
        description={`${status.name} · write generated manifests for this stage`}
        actions={
          <Group gap="sm">
            <Button component={Link} to={`/deployables/${status.name}`} variant="default">
              Back
            </Button>
            <Button
              disabled={!hasChanges}
              onClick={() => {
                setSyncArgo(true);
                reconcileHandlers.open();
              }}
            >
              Reconcile
            </Button>
          </Group>
        }
      />

      <MutationModeAlert apply={status.apply} push={status.push} />

      {!hasChanges ? (
        <Alert color="gray" title="Already reconciled">
          Generated manifests for <strong>{params.stage}</strong> match the ops repo.
        </Alert>
      ) : (
        <Text size="sm" c="dimmed">
          {files.length} file{files.length === 1 ? "" : "s"} will change. Shared workload
          base is included; other stage overlays are left alone.
        </Text>
      )}

      {files.length > 0 ? (
        <Stack gap={4}>
          {files.map((file) => (
            <Text key={file} size="xs" ff="monospace">
              {file}
            </Text>
          ))}
        </Stack>
      ) : null}

      <DiffPanel diff={preview.diff ?? ""} title="Reconcile preview" maxHeight="60vh" />

      <ConfirmActionModal
        opened={reconcileOpen}
        onClose={reconcileHandlers.close}
        loading={committing}
        title={`Reconcile ${status.name}`}
        confirmLabel={mutationCommitLabel(status, "reconcile", status.sync && syncArgo)}
        confirmDisabled={!hasChanges}
        argoURL={stageArgoURL}
        message={
          <Stack gap="sm">
            <Text size="sm">
              Write generated manifests for <strong>{params.stage}</strong> (
              {files.length} file{files.length === 1 ? "" : "s"}). Shared workload base is
              included; other stage overlays are left alone.
            </Text>
            <ArgoSyncCheckbox
              show={status.apply && status.sync}
              checked={syncArgo}
              onChange={setSyncArgo}
              stage={params.stage}
            />
            <MutationGitHint
              apply={status.apply}
              push={status.push}
              sync={status.sync && syncArgo}
              syncStage={params.stage}
            />
          </Stack>
        }
        onConfirm={() => {
          void fetcher.submit(
            {
              intent: "reconcile",
              ...(status.sync ? { sync: syncArgo ? "true" : "false" } : {}),
              ...(status.apply ? { wait: "false" } : {}),
            },
            { method: "post" },
          );
        }}
      />
    </Stack>
  );
}

function MutationModeAlert({ apply, push }: { apply: boolean; push: boolean }) {
  if (!apply) {
    return (
      <Alert color="yellow" title="Dry-run">
        Preview only. Start the API with{" "}
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
        Reconcile commits locally and does not push. Start the API with{" "}
        <Text span ff="monospace">
          --push
        </Text>{" "}
        to update the ops remote. Never force-pushes.
      </Alert>
    );
  }
  return null;
}
