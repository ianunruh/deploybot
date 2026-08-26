import { Fragment } from "react";
import { Badge, Group, Stack, Text } from "@mantine/core";
import { IconArrowRight } from "@tabler/icons-react";

import type { Flow, FlowHop, StageStatus } from "~/lib/api.server";
import { CompactImage } from "~/ui/compact-image";
import { ConsolePaper } from "~/ui/console-paper";
import { SourceCommitMeta } from "~/ui/source-commit";
import { StatusBadge } from "~/ui/status-badge";

const HOP_LABEL: Record<string, string> = {
  caught_up: "caught up",
  dest_ahead: "dest is newer",
  source_unhealthy: "source not healthy",
  baking: "baking",
  waiting_approval: "waiting for approval",
  ready: "ready to promote",
  behind: "behind",
};

const HOP_COLOR: Record<string, string> = {
  caught_up: "teal",
  dest_ahead: "yellow",
  source_unhealthy: "red",
  baking: "yellow",
  waiting_approval: "accent",
  ready: "teal",
  behind: "gray",
};

function hopLabel(hop: FlowHop): string {
  if (hop.state === "baking" && hop.remaining) return `bake ${hop.remaining}`;
  return HOP_LABEL[hop.state] ?? hop.state;
}

function hopColor(hop: FlowHop): string {
  return HOP_COLOR[hop.state] ?? "gray";
}

export function ReleaseFlow({ stages, flow }: { stages: StageStatus[]; flow?: Flow }) {
  if (stages.length === 0) return null;
  const hops = flow?.hops ?? [];
  return (
    <Stack gap="sm">
      {flow?.source || flow?.tag || flow?.image ? (
        <Group justify="space-between" align="baseline">
          <SourceCommitMeta source={flow?.source} />
          {flow?.tag || flow?.image ? (
            <CompactImage value={flow.tag || flow.image} />
          ) : null}
        </Group>
      ) : null}
      <ConsolePaper>
        <div className="db-flow">
          {stages.map((st, i) => {
            const hop = hops.find(
              (h) => h.from === st.name && h.to === stages[i + 1]?.name,
            );
            return (
              <Fragment key={st.name}>
                <Stack gap={4} className="db-flow-node">
                  <Text size="sm" fw={600}>
                    {st.name}
                  </Text>
                  <CompactImage value={st.image} empty="unpinned" />
                  <StatusBadge status={st.health} href={st.argoURL} />
                </Stack>
                {i < stages.length - 1 ? (
                  <div className="db-flow-edge">
                    <div className="db-flow-edge-rule" />
                    {hop ? (
                      <Badge size="sm" variant="light" color={hopColor(hop)} tt="none">
                        {hopLabel(hop)}
                      </Badge>
                    ) : null}
                    <IconArrowRight
                      size={16}
                      stroke={1.5}
                      color="var(--mantine-color-dimmed)"
                    />
                    <div className="db-flow-edge-rule" />
                  </div>
                ) : null}
              </Fragment>
            );
          })}
        </div>
      </ConsolePaper>
    </Stack>
  );
}

export function ReleaseFlowInline({
  stages,
  flow,
}: {
  stages: StageStatus[];
  flow?: Flow;
}) {
  if (stages.length === 0) {
    return (
      <Text size="sm" c="dimmed">
        —
      </Text>
    );
  }
  const hops = flow?.hops ?? [];
  return (
    <div className="db-flow-inline">
      {stages.map((st, i) => {
        const hop = hops.find((h) => h.from === st.name && h.to === stages[i + 1]?.name);
        return (
          <Fragment key={st.name}>
            <Text size="sm" fw={600}>
              {st.name}
            </Text>
            {i < stages.length - 1 ? (
              <span className="db-flow-inline-edge">
                {hop ? (
                  <Badge size="sm" variant="light" color={hopColor(hop)} tt="none">
                    {hopLabel(hop)}
                  </Badge>
                ) : null}
                <IconArrowRight
                  size={14}
                  stroke={1.5}
                  color="var(--mantine-color-dimmed)"
                />
              </span>
            ) : null}
          </Fragment>
        );
      })}
    </div>
  );
}
