import { Anchor, Badge, Group, Stack, Text } from "@mantine/core";

import type { Release, ReleaseStageHit } from "~/lib/api.server";
import { CompactImage } from "~/ui/compact-image";
import { RelativeTime } from "~/ui/relative-time";
import { ResourceTable, Table } from "~/ui/resource-table";

export function ReleaseHistory({
  stages,
  releases,
  error,
}: {
  stages: string[];
  releases: Release[];
  error?: string | null;
}) {
  return (
    <Stack gap="sm">
      <Text size="sm" tt="uppercase" c="dimmed" fw={600}>
        Releases
      </Text>
      {error != null ? (
        <Text size="sm" c="dimmed">
          Could not load history ({error}).
        </Text>
      ) : (
        <ResourceTable
          headers={["Image", ...stages]}
          isEmpty={releases.length === 0}
          emptyMessage="No pin or promote commits yet."
          minWidth={Math.max(480, 180 + stages.length * 160)}
        >
          {releases.map((rel) => (
            <Table.Tr key={rel.digest || rel.image}>
              <Table.Td className="db-cell-clip">
                <Group gap="xs" wrap="nowrap">
                  <CompactImage value={rel.image} />
                  {rel.current ? (
                    <Badge size="xs" variant="light" color="accent" tt="none">
                      current
                    </Badge>
                  ) : null}
                </Group>
              </Table.Td>
              {stages.map((stage) => (
                <Table.Td key={stage} className="db-cell-fit">
                  <ReleaseStageCell hit={rel.stages[stage]} />
                </Table.Td>
              ))}
            </Table.Tr>
          ))}
        </ResourceTable>
      )}
    </Stack>
  );
}

function ReleaseStageCell({ hit }: { hit?: ReleaseStageHit }) {
  if (hit == null) {
    return (
      <Text size="xs" c="dimmed">
        —
      </Text>
    );
  }
  return (
    <Stack gap={0}>
      <Group gap={6} wrap="nowrap">
        <Text size="xs">{hit.kind}</Text>
        {hit.commitURL ? (
          <Anchor
            href={hit.commitURL}
            size="xs"
            target="_blank"
            rel="noreferrer"
            c="accent.4"
          >
            commit
          </Anchor>
        ) : null}
      </Group>
      <RelativeTime value={hit.at} size="xs" />
    </Stack>
  );
}
