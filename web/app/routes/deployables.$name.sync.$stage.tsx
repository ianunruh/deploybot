import { Alert, Button, Group, Stack, Text } from "@mantine/core";
import { Link, useFetcher, useRevalidator } from "react-router";

import type { Route } from "./+types/deployables.$name.sync.$stage";
import {
  getDeployable,
  previewSync,
  syncDeployable,
  type MutationResult,
} from "~/lib/api.server";
import { notifyActionError, notifyActionSuccess } from "~/lib/action-feedback";
import { useFetcherResult } from "~/lib/use-fetcher-result";
import { DiffPanel } from "~/ui/diff-panel";
import { PageHeader } from "~/ui/page-header";

export function meta({ params }: Route.MetaArgs) {
  return [{ title: `Sync ${params.stage} · ${params.name} · deploybot` }];
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
      previewSync(name, stage),
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
  if (String(form.get("intent") ?? "") !== "sync") {
    return { ok: false, error: "unknown intent" } satisfies ActionData;
  }
  try {
    return { ok: true, result: await syncDeployable(name, stage) } satisfies ActionData;
  } catch (err) {
    return {
      ok: false,
      error: err instanceof Error ? err.message : String(err),
    } satisfies ActionData;
  }
}

function syncCrumbs(name?: string) {
  const crumbs = [{ label: "Deployables", to: "/" }];
  if (name) {
    crumbs.push({ label: name, to: `/deployables/${name}` });
  }
  return crumbs;
}

export default function SyncStage({ loaderData, params }: Route.ComponentProps) {
  const { status, preview, error } = loaderData;
  const revalidator = useRevalidator();
  const fetcher = useFetcher<ActionData>();

  useFetcherResult(fetcher, (data) => {
    if (!data.ok) {
      notifyActionError("Sync failed", data.error);
      return;
    }
    notifyActionSuccess("Sync", `Wrote manifests${mutationNote(data.result)}`);
    void revalidator.revalidate();
  });

  if (error != null || status == null || preview == null) {
    return (
      <Stack gap="lg">
        <PageHeader title="Sync" crumbs={syncCrumbs(params.name)} />
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
  const canCommit = status.apply && hasChanges && !committing;

  return (
    <Stack gap="lg">
      <PageHeader
        title={`Sync ${params.stage}`}
        crumbs={syncCrumbs(status.name)}
        description={`${status.name} · write generated manifests for this stage`}
        actions={
          <Group gap="sm">
            <Button component={Link} to={`/deployables/${status.name}`} variant="default">
              Back
            </Button>
            <Button
              disabled={!canCommit}
              loading={committing}
              onClick={() => {
                void fetcher.submit({ intent: "sync" }, { method: "post" });
              }}
            >
              {commitLabel(status)}
            </Button>
          </Group>
        }
      />

      <MutationModeAlert apply={status.apply} push={status.push} />

      {!hasChanges ? (
        <Alert color="gray" title="Already in sync">
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

      <DiffPanel diff={preview.diff ?? ""} title="Sync preview" maxHeight="60vh" />
    </Stack>
  );
}

function commitLabel(status: { apply: boolean; push: boolean }): string {
  if (!status.apply) return "API is dry-run";
  if (status.push) return "Commit and push sync";
  return "Commit sync";
}

function mutationNote(result: MutationResult): string {
  if (result.dryRun) return " (dry-run)";
  if (result.pushed) return " and pushed";
  return "";
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
        Sync commits locally and does not push. Start the API with{" "}
        <Text span ff="monospace">
          --push
        </Text>{" "}
        to update the ops remote. Never force-pushes.
      </Alert>
    );
  }
  return null;
}
