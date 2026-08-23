import { Anchor, Group, Stack, Text } from "@mantine/core";

import type { SourceCommit } from "~/lib/api.server";

export function SourceCommitMeta({ source }: { source?: SourceCommit }) {
  if (source == null || (!source.sha && !source.message && !source.author)) {
    return null;
  }
  const sha = source.sha ? source.sha.slice(0, 7) : "";
  const shaEl =
    sha && source.url ? (
      <Anchor
        href={source.url}
        size="xs"
        ff="monospace"
        target="_blank"
        rel="noreferrer"
        c="accent.4"
      >
        {sha}
      </Anchor>
    ) : sha ? (
      <Text size="xs" ff="monospace" c="dimmed">
        {sha}
      </Text>
    ) : null;
  return (
    <Stack gap={0}>
      {source.message ? (
        <Text size="sm" lineClamp={1} title={source.message}>
          {source.message}
        </Text>
      ) : null}
      {source.author || shaEl ? (
        <Group gap={6} wrap="nowrap">
          {source.author ? (
            <Text size="xs" c="dimmed" lineClamp={1}>
              {source.author}
            </Text>
          ) : null}
          {source.author && shaEl ? (
            <Text size="xs" c="dimmed">
              ·
            </Text>
          ) : null}
          {shaEl}
        </Group>
      ) : null}
    </Stack>
  );
}
