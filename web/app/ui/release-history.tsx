import { Anchor, Badge, Button, Group, Stack, Text } from "@mantine/core";

import type { Release, ReleaseStageHit } from "~/lib/api.server";
import { CompactImage, releaseImageRef, releaseMatchesImage } from "~/ui/compact-image";
import { HistoryActor } from "~/ui/history-actor";
import { RelativeTime } from "~/ui/relative-time";
import { ResourceTable, Table } from "~/ui/resource-table";
import { SourceCommitMeta } from "~/ui/source-commit";

export function ReleaseHistory({
  stages,
  releases,
  imageRepo,
  stageImages,
  error,
  onRollback,
}: {
  stages: string[];
  releases: Release[];
  imageRepo?: string;
  stageImages?: Record<string, string | undefined>;
  error?: string | null;
  onRollback?: (stage: string, image: string) => void;
}) {
  return (
    <Stack gap="sm">
      {error != null ? (
        <Text size="sm" c="dimmed">
          Could not load history ({error}).
        </Text>
      ) : (
        <ResourceTable
          headers={["Image", ...stages]}
          isEmpty={releases.length === 0}
          emptyMessage="No pin or promote commits yet."
          minWidth={Math.max(560, 260 + stages.length * 160)}
        >
          {releases.map((rel) => (
            <Table.Tr key={rel.digest || rel.image}>
              <Table.Td className="db-cell-clip">
                <Stack gap={4}>
                  <Group gap="xs" wrap="nowrap">
                    <CompactImage value={rel.image} />
                    {rel.current ? (
                      <Badge size="xs" variant="light" color="accent" tt="none">
                        current
                      </Badge>
                    ) : null}
                  </Group>
                  <SourceCommitMeta source={rel.source} />
                </Stack>
              </Table.Td>
              {stages.map((stage) => (
                <Table.Td key={stage} className="db-cell-fit">
                  <ReleaseStageCell
                    hit={rel.stages[stage]}
                    canRollback={
                      onRollback != null &&
                      imageRepo != null &&
                      rel.stages[stage] != null &&
                      !releaseMatchesImage(rel, stageImages?.[stage])
                    }
                    onRollback={() => {
                      if (!imageRepo || onRollback == null) return;
                      onRollback(stage, releaseImageRef(imageRepo, rel));
                    }}
                  />
                </Table.Td>
              ))}
            </Table.Tr>
          ))}
        </ResourceTable>
      )}
    </Stack>
  );
}

function ReleaseStageCell({
  hit,
  canRollback,
  onRollback,
}: {
  hit?: ReleaseStageHit;
  canRollback?: boolean;
  onRollback?: () => void;
}) {
  if (hit == null) {
    return (
      <Text size="xs" c="dimmed">
        —
      </Text>
    );
  }
  return (
    <Stack gap={4}>
      <Stack gap={0}>
        <Group gap={6} wrap="nowrap">
          <Text size="xs">{hit.kind}</Text>
          {hit.commitURL ? (
            <Anchor
              href={hit.commitURL}
              size="xs"
              target="_blank"
              rel="noreferrer"
              c="var(--db-link)"
            >
              commit
            </Anchor>
          ) : null}
        </Group>
        <RelativeTime value={hit.at} size="xs" />
      </Stack>
      <HistoryActor actor={hit.actor} author={hit.author} size="xs" />
      {canRollback ? (
        <Button variant="default" size="compact-xs" onClick={onRollback}>
          Rollback
        </Button>
      ) : null}
    </Stack>
  );
}
