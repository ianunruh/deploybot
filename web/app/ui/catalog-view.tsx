import {
  ActionIcon,
  Anchor,
  Badge,
  Box,
  Grid,
  Group,
  Stack,
  Text,
  Tooltip,
} from "@mantine/core";
import { IconExternalLink, IconLayoutGrid, IconTable } from "@tabler/icons-react";
import type { KeyboardEvent } from "react";
import { Link, useNavigate } from "react-router";

import type { DeployableSummary, PauseFile, StageStatus } from "~/lib/api.server";
import { visiblePauses } from "~/lib/pause";
import { formatRelative } from "~/lib/time";
import { ConsolePaper } from "~/ui/console-paper";
import { DeployableLinkIcons, ObservabilityClusterMenus } from "~/ui/external-links";
import { RelativeTime } from "~/ui/relative-time";
import { ReleaseFlowInline } from "~/ui/release-flow";
import { updatesNameHref } from "~/ui/resource-filter";
import { ResourceTable, Table } from "~/ui/resource-table";
import { PauseBadge, UpdateBadge } from "~/ui/status-badge";

export type CatalogViewMode = "cards" | "table";

export function CatalogViewToggle({
  value,
  onChange,
}: {
  value: CatalogViewMode;
  onChange: (view: CatalogViewMode) => void;
}) {
  return (
    <ActionIcon.Group>
      <ActionIcon
        variant={value === "cards" ? "filled" : "default"}
        color={value === "cards" ? "accent" : "gray"}
        size="sm"
        aria-label="Card view"
        title="Cards"
        aria-pressed={value === "cards"}
        onClick={() => onChange("cards")}
      >
        <IconLayoutGrid size={16} />
      </ActionIcon>
      <ActionIcon
        variant={value === "table" ? "filled" : "default"}
        color={value === "table" ? "accent" : "gray"}
        size="sm"
        aria-label="Table view"
        title="Table"
        aria-pressed={value === "table"}
        onClick={() => onChange("table")}
      >
        <IconTable size={16} />
      </ActionIcon>
    </ActionIcon.Group>
  );
}

export function CatalogView({
  deployables,
  view,
  pause,
  emptyMessage = "No deployable specs found.",
}: {
  deployables: DeployableSummary[];
  view: CatalogViewMode;
  pause?: PauseFile;
  emptyMessage?: string;
}) {
  if (deployables.length === 0) {
    return (
      <Text c="dimmed" size="sm" py="xl" ta="center">
        {emptyMessage}
      </Text>
    );
  }
  if (view === "table") {
    return <CatalogTable deployables={deployables} pause={pause} />;
  }
  return (
    <Stack gap="xl">
      {groupByNamespace(deployables).map((section) => (
        <Stack key={section.namespace} gap="sm">
          <Text size="sm" tt="uppercase" c="dimmed" fw={600}>
            {section.namespace} · {section.items.length}
          </Text>
          <Grid gap="md" align="start">
            {section.items.map((d) => (
              <Grid.Col key={d.name} span={{ base: 12, sm: 6, lg: 4 }}>
                <CatalogCard deployable={d} pause={pause} />
              </Grid.Col>
            ))}
          </Grid>
        </Stack>
      ))}
    </Stack>
  );
}

function CatalogTable({
  deployables,
  pause,
}: {
  deployables: DeployableSummary[];
  pause?: PauseFile;
}) {
  return (
    <ResourceTable
      headers={["Name", "Project", "Namespace", "Flow", "Last deploy", "Links"]}
      isEmpty={false}
    >
      {deployables.map((d) => (
        <Table.Tr key={d.name}>
          <Table.Td>
            <Stack gap={2}>
              <Group gap="xs" wrap="nowrap">
                <Text
                  component={Link}
                  to={`/deployables/${d.name}`}
                  fw={600}
                  c="var(--db-link)"
                >
                  {d.name}
                </Text>
                {catalogPaused(pause, d) ? <PauseBadge /> : null}
                {d.update?.stale ? (
                  <UpdateBadge stale to={updatesNameHref(d.name)} />
                ) : null}
              </Group>
              {d.summary ? (
                <Text size="xs" c="dimmed">
                  {d.summary}
                </Text>
              ) : null}
            </Stack>
          </Table.Td>
          <Table.Td>
            {d.project ? (
              <Badge variant="light" size="sm" tt="uppercase" radius="sm">
                {d.project}
              </Badge>
            ) : (
              <Text size="sm" c="dimmed">
                —
              </Text>
            )}
          </Table.Td>
          <Table.Td>{d.namespace}</Table.Td>
          <Table.Td>
            <ReleaseFlowInline stages={d.stages ?? []} flow={d.flow} />
          </Table.Td>
          <Table.Td className="db-cell-fit">
            <RelativeTime value={d.deployedAt} />
          </Table.Td>
          <Table.Td>
            {d.repoURL ||
            d.projectURL ||
            d.docsURL ||
            (d.stages ?? []).some(
              (st) => st.headlampURL || st.grafanaURL || st.logsURL,
            ) ? (
              <Group gap={2} wrap="nowrap">
                <DeployableLinkIcons
                  repoURL={d.repoURL}
                  projectURL={d.projectURL}
                  docsURL={d.docsURL}
                />
                <ObservabilityClusterMenus stages={d.stages ?? []} />
              </Group>
            ) : (
              <Text size="sm" c="dimmed">
                —
              </Text>
            )}
          </Table.Td>
        </Table.Tr>
      ))}
    </ResourceTable>
  );
}

function CatalogCard({
  deployable: d,
  pause,
}: {
  deployable: DeployableSummary;
  pause?: PauseFile;
}) {
  const navigate = useNavigate();
  const href = `/deployables/${d.name}`;
  const open = openHref(d.stages ?? []);

  function go() {
    void navigate(href);
  }

  function onKeyDown(event: KeyboardEvent<HTMLDivElement>) {
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      go();
    }
  }

  return (
    <ConsolePaper
      className="db-catalog-card"
      role="link"
      tabIndex={0}
      aria-label={d.name}
      onClick={go}
      onKeyDown={onKeyDown}
    >
      <Stack gap="xs">
        <Group justify="space-between" align="flex-start" wrap="nowrap" gap="sm">
          <Stack gap={4} style={{ minWidth: 0, flex: 1 }}>
            <Group gap="xs" wrap="nowrap">
              <Text fw={600} truncate>
                {d.name}
              </Text>
              {catalogPaused(pause, d) ? <PauseBadge /> : null}
              {d.update?.stale ? (
                <UpdateBadge stale to={updatesNameHref(d.name)} />
              ) : null}
            </Group>
            <Text size="sm" c="dimmed" lineClamp={2}>
              {d.summary || d.namespace}
            </Text>
          </Stack>
          {open ? (
            <Tooltip label={open.hostname} withArrow>
              <Anchor
                href={open.href}
                target="_blank"
                rel="noreferrer"
                size="sm"
                c="var(--db-link)"
                display="inline-flex"
                style={{ alignItems: "center", gap: 6, flexShrink: 0 }}
                onClick={(event) => event.stopPropagation()}
              >
                Open
                <IconExternalLink size={12} />
              </Anchor>
            </Tooltip>
          ) : null}
        </Group>
        <Group justify="space-between" align="center" wrap="nowrap" gap="sm">
          <StageDots stages={d.stages ?? []} />
          <RelativeTime value={d.deployedAt} size="xs" />
        </Group>
      </Stack>
    </ConsolePaper>
  );
}

function StageDots({ stages }: { stages: StageStatus[] }) {
  if (stages.length === 0) {
    return (
      <Text size="xs" c="dimmed">
        —
      </Text>
    );
  }
  return (
    <Group gap="sm" wrap="nowrap">
      {stages.map((st) => (
        <Tooltip key={st.name} label={stageDotLabel(st)} withArrow>
          <Group gap={6} wrap="nowrap">
            <Box
              className="db-stage-dot"
              bg={
                st.connected === false
                  ? "var(--mantine-color-orange-6)"
                  : healthDotColor(st.health)
              }
            />
            <Text size="xs" c="dimmed">
              {st.name}
            </Text>
          </Group>
        </Tooltip>
      ))}
    </Group>
  );
}

function openHref(stages: StageStatus[]): { href: string; hostname: string } | null {
  const withHost = stages.filter((st) => st.hostname);
  const st = withHost.at(-1);
  if (st == null || !st.hostname) return null;
  const href = /^https?:\/\//i.test(st.hostname) ? st.hostname : `https://${st.hostname}`;
  return { href, hostname: st.hostname };
}

function groupByNamespace(
  items: DeployableSummary[],
): { namespace: string; items: DeployableSummary[] }[] {
  const buckets = new Map<string, DeployableSummary[]>();
  for (const item of items) {
    const namespace = item.namespace?.trim() || "other";
    const list = buckets.get(namespace) ?? [];
    list.push(item);
    buckets.set(namespace, list);
  }
  return [...buckets.keys()]
    .sort((a, b) => a.localeCompare(b))
    .map((namespace) => ({ namespace, items: buckets.get(namespace) ?? [] }));
}

function stageDotLabel(st: StageStatus): string {
  const health = st.health || "unknown";
  if (st.connected === false) {
    const seen = st.updatedAt ? ` last seen ${formatRelative(st.updatedAt)}` : "";
    return `${st.name}: ${health} (unreachable${seen})`;
  }
  return `${st.name}: ${health}`;
}

function catalogPaused(pause: PauseFile | undefined, d: DeployableSummary): boolean {
  return (
    visiblePauses(
      pause,
      d.name,
      (d.stages ?? []).map((st) => st.name),
    ).length > 0
  );
}

function healthDotColor(status?: string): string {
  const key = (status ?? "unknown").toLowerCase();
  if (key === "healthy" || key === "synced" || key === "running") {
    return "var(--mantine-color-teal-6)";
  }
  if (key === "degraded" || key === "missing" || key === "failed") {
    return "var(--mantine-color-red-6)";
  }
  if (
    key === "progressing" ||
    key === "outofsync" ||
    key === "pending" ||
    key === "notready"
  ) {
    return "var(--mantine-color-yellow-6)";
  }
  return "var(--mantine-color-gray-5)";
}
