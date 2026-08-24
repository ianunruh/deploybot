import { Anchor, Group, Stack, Text } from "@mantine/core";

import type { Workflows } from "~/lib/api.server";
import { RelativeTime } from "~/ui/relative-time";
import { ResourceTable, Table } from "~/ui/resource-table";
import { SourceCommitMeta } from "~/ui/source-commit";
import { StatusBadge } from "~/ui/status-badge";

export function WorkflowRuns({ workflows }: { workflows?: Workflows | null }) {
  if (workflows == null) return null;

  const runs = workflows.runs ?? [];
  const error = workflows.error;

  return (
    <Stack gap="sm">
      <Group justify="space-between" align="baseline">
        <Text size="sm" tt="uppercase" c="dimmed" fw={600}>
          Workflows
        </Text>
        {workflows.url ? (
          <Anchor
            href={workflows.url}
            size="xs"
            c="dimmed"
            target="_blank"
            rel="noreferrer"
          >
            GitHub Actions
          </Anchor>
        ) : null}
      </Group>
      {error != null && error !== "" ? (
        <Text size="sm" c="dimmed">
          Could not load workflow runs ({error}).
        </Text>
      ) : (
        <ResourceTable
          headers={["Status", "Workflow", "Title", "Branch", "Event", "Actor", "Started"]}
          isEmpty={runs.length === 0}
          emptyMessage="No recent workflow runs."
          minWidth={880}
        >
          {runs.map((run) => (
            <Table.Tr key={run.id}>
              <Table.Td className="db-cell-fit">
                <StatusBadge status={run.status} />
              </Table.Td>
              <Table.Td className="db-cell-fit">
                <WorkflowName name={run.name} number={run.number} url={run.url} />
              </Table.Td>
              <Table.Td className="db-cell-clip">
                <SourceCommitMeta
                  source={{
                    sha: run.sha,
                    message: run.title,
                    url: run.commitURL,
                  }}
                />
              </Table.Td>
              <Table.Td className="db-cell-fit">
                <Text size="sm" ff="monospace">
                  {run.branch || "—"}
                </Text>
              </Table.Td>
              <Table.Td className="db-cell-fit">{run.event || "—"}</Table.Td>
              <Table.Td className="db-cell-fit">{run.actor || "—"}</Table.Td>
              <Table.Td className="db-cell-fit">
                <RelativeTime value={run.startedAt} size="xs" />
              </Table.Td>
            </Table.Tr>
          ))}
        </ResourceTable>
      )}
    </Stack>
  );
}

function WorkflowName({
  name,
  number,
  url,
}: {
  name: string;
  number: number;
  url?: string;
}) {
  const label = number > 0 ? `${name} #${number}` : name;
  if (url) {
    return (
      <Anchor href={url} size="sm" target="_blank" rel="noreferrer" c="var(--db-link)">
        {label}
      </Anchor>
    );
  }
  return <Text size="sm">{label}</Text>;
}
