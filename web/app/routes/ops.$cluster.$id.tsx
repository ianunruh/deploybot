import { Alert, Code, Stack, Text } from "@mantine/core";
import { useEffect } from "react";
import { useRevalidator } from "react-router";

import type { Route } from "./+types/ops.$cluster.$id";
import { getOpsExecution, type OpsExecution } from "~/lib/api.server";
import { ConsolePaper } from "~/ui/console-paper";
import { HistoryActor } from "~/ui/history-actor";
import { OpsLog } from "~/ui/ops-log";
import { PageHeader } from "~/ui/page-header";
import { RelativeTime } from "~/ui/relative-time";
import { StatusBadge } from "~/ui/status-badge";

export function meta({ params }: Route.MetaArgs) {
  return [{ title: `${params.id ?? "execution"} · Ops · deploybot` }];
}

export async function loader({ params }: Route.LoaderArgs) {
  const cluster = params.cluster ?? "";
  const id = params.id ?? "";
  try {
    const execution = await getOpsExecution(cluster, id);
    return { execution, error: null as string | null };
  } catch (err) {
    return {
      execution: null as OpsExecution | null,
      error: err instanceof Error ? err.message : String(err),
    };
  }
}

export default function OpsExecution({ loaderData, params }: Route.ComponentProps) {
  const { execution, error } = loaderData;
  const revalidator = useRevalidator();
  const live = execution?.phase === "Pending" || execution?.phase === "Running";

  useEffect(() => {
    if (!live) return;
    const id = window.setInterval(() => {
      if (revalidator.state === "idle") void revalidator.revalidate();
    }, 3_000);
    return () => window.clearInterval(id);
  }, [live, revalidator]);

  const cluster = params.cluster ?? execution?.cluster ?? "";
  const id = params.id ?? execution?.id ?? "";

  return (
    <Stack gap="lg">
      <PageHeader
        title={execution?.summary || id}
        description={
          execution ? (
            <Stack gap={4}>
              <Text size="sm" c="dimmed">
                {execution.kind} on {execution.cluster}
                {execution.dryRun ? " · dry-run" : ""}
                {execution.ref ? ` · ${execution.ref}` : ""}
              </Text>
              <StatusBadge status={execution.phase} />
            </Stack>
          ) : (
            "Execution detail"
          )
        }
        crumbs={[{ label: "Ops", to: "/ops" }, { label: id }]}
      />
      {error != null && (
        <Alert color="red" title="Could not load execution">
          {error}
        </Alert>
      )}
      {execution != null ? (
        <ConsolePaper>
          <Stack gap="xs">
            <Text size="xs" c="dimmed">
              {execution.id}
              {execution.podName ? ` · ${execution.podName}` : ""}
              {execution.createdAt ? (
                <>
                  {" · "}
                  <RelativeTime value={execution.createdAt} />
                </>
              ) : null}
            </Text>
            <HistoryActor actor={execution.actor} />
            {execution.message ? (
              <Text size="sm" c="dimmed">
                {execution.message}
              </Text>
            ) : null}
            {execution.command && execution.command.length > 0 ? (
              <Code block>{execution.command.join(" ")}</Code>
            ) : null}
          </Stack>
        </ConsolePaper>
      ) : null}
      <ConsolePaper>
        <OpsLog
          key={`${cluster}:${id}`}
          src={`/ops/${cluster}/${id}/logs`}
          active={live}
        />
      </ConsolePaper>
    </Stack>
  );
}
