import { Anchor, Group, Stack, Text } from "@mantine/core";

import type { Changelog } from "~/lib/api.server";
import { SourceCommitMeta } from "~/ui/source-commit";

export function PromoteChangelog({
  changelog,
  error,
  loading,
  to,
}: {
  changelog?: Changelog | null;
  error?: string | null;
  loading?: boolean;
  to?: string;
}) {
  if (loading) {
    return (
      <Text size="sm" c="dimmed">
        Loading changelog…
      </Text>
    );
  }
  if (error) {
    return (
      <Text size="sm" c="dimmed">
        Could not load changelog ({error}).
      </Text>
    );
  }
  if (changelog == null || !hasChangelog(changelog)) {
    return null;
  }
  const heading = changelogHeading(changelog, to);
  const commits = changelog.commits ?? [];
  return (
    <Stack gap="xs">
      <Group justify="space-between" align="baseline" gap="sm" wrap="nowrap">
        {heading ? (
          <Text size="sm" fw={600}>
            {heading}
          </Text>
        ) : (
          <Text size="sm" c="dimmed">
            {changelog.url ? "Commits" : "Source commit"}
          </Text>
        )}
        {changelog.url ? (
          <Anchor
            href={changelog.url}
            size="xs"
            target="_blank"
            rel="noreferrer"
            c="var(--db-link)"
          >
            Compare on GitHub
          </Anchor>
        ) : null}
      </Group>
      {changelog.error ? (
        <Text size="sm" c="dimmed">
          Could not list commits ({changelog.error}).
        </Text>
      ) : null}
      {commits.length > 0 ? (
        <div className="db-changelog">
          {commits.map((c, i) => (
            <div key={c.sha || String(i)} className="db-changelog-row">
              <SourceCommitMeta source={c} />
            </div>
          ))}
        </div>
      ) : null}
    </Stack>
  );
}

function hasChangelog(cl: Changelog): boolean {
  return Boolean(
    cl.url ||
    cl.status ||
    cl.error ||
    (cl.commits && cl.commits.length > 0) ||
    cl.head?.sha,
  );
}

function changelogHeading(cl: Changelog, to?: string): string {
  if (cl.status === "identical") {
    return "No new commits";
  }
  if (cl.status === "behind") {
    const n = cl.behindBy ?? 0;
    const dest = to || cl.to || "dest";
    return n === 1 ? `${dest} is 1 commit ahead` : `${dest} is ${n} commits ahead`;
  }
  const n = cl.aheadBy ?? cl.commits?.length ?? 0;
  if (n === 0) {
    return "";
  }
  if (cl.status === "diverged") {
    const behind = cl.behindBy ?? 0;
    const ahead = n === 1 ? "1 commit ahead" : `${n} commits ahead`;
    if (behind === 1) return `${ahead}, 1 behind`;
    if (behind > 1) return `${ahead}, ${behind} behind`;
    return ahead;
  }
  if (cl.truncated && cl.aheadBy != null && cl.aheadBy > (cl.commits?.length ?? 0)) {
    return `Showing ${cl.commits?.length ?? 0} of ${cl.aheadBy} commits`;
  }
  return n === 1 ? "1 commit" : `${n} commits`;
}
