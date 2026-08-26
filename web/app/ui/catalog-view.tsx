import {
  ActionIcon,
  Anchor,
  Avatar,
  Badge,
  Box,
  Group,
  SimpleGrid,
  Stack,
  Text,
  Tooltip,
} from "@mantine/core";
import { IconExternalLink, IconLayoutGrid, IconTable } from "@tabler/icons-react";
import type { KeyboardEvent } from "react";
import { Link, useNavigate } from "react-router";

import type { DeployableSummary, StageStatus } from "~/lib/api.server";
import { ConsolePaper } from "~/ui/console-paper";
import { DeployableLinkIcons, ObservabilityClusterMenus } from "~/ui/external-links";
import { RelativeTime } from "~/ui/relative-time";
import { ReleaseFlowInline } from "~/ui/release-flow";
import { ResourceTable, Table } from "~/ui/resource-table";
import { UpdateBadge } from "~/ui/status-badge";

export type CatalogViewMode = "cards" | "table";

const GROUP_ORDER = ["play", "platform"];

const AVATAR_COLORS = [
  "accent",
  "teal",
  "violet",
  "cyan",
  "grape",
  "indigo",
  "orange",
  "pink",
] as const;

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
}: {
  deployables: DeployableSummary[];
  view: CatalogViewMode;
}) {
  if (deployables.length === 0) {
    return (
      <Text c="dimmed" size="sm" py="xl" ta="center">
        No deployable specs found.
      </Text>
    );
  }
  if (view === "table") {
    return <CatalogTable deployables={deployables} />;
  }
  return (
    <Stack gap="xl">
      {groupDeployables(deployables).map((section) => (
        <Stack key={section.group} gap="sm">
          <Text size="sm" tt="uppercase" c="dimmed" fw={600}>
            {groupLabel(section.group)} · {section.items.length}
          </Text>
          <SimpleGrid cols={{ base: 1, sm: 2, lg: 3 }} spacing="md">
            {section.items.map((d) => (
              <CatalogCard key={d.name} deployable={d} />
            ))}
          </SimpleGrid>
        </Stack>
      ))}
    </Stack>
  );
}

function CatalogTable({ deployables }: { deployables: DeployableSummary[] }) {
  return (
    <ResourceTable
      headers={["Name", "Group", "Namespace", "Flow", "Last deploy", "Links"]}
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
                {d.update?.stale ? <UpdateBadge stale /> : null}
              </Group>
              {d.summary ? (
                <Text size="xs" c="dimmed">
                  {d.summary}
                </Text>
              ) : null}
            </Stack>
          </Table.Td>
          <Table.Td>
            {d.group ? (
              <Badge variant="light" size="sm" tt="uppercase" radius="sm">
                {d.group}
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

function CatalogCard({ deployable: d }: { deployable: DeployableSummary }) {
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
        <Group align="flex-start" wrap="nowrap" gap="sm">
          <CatalogAvatar name={d.name} icon={d.icon} />
          <Stack gap={4} style={{ minWidth: 0, flex: 1 }}>
            <Group gap="xs" wrap="nowrap" justify="space-between">
              <Text fw={600} truncate>
                {d.name}
              </Text>
              {d.update?.stale ? <UpdateBadge stale /> : null}
            </Group>
            <Text size="sm" c="dimmed" lineClamp={2}>
              {d.summary || d.namespace}
            </Text>
          </Stack>
        </Group>
        <Group justify="space-between" align="center" wrap="nowrap" gap="sm">
          <StageDots stages={d.stages ?? []} />
          <RelativeTime value={d.deployedAt} size="xs" />
        </Group>
        {open ? (
          <Tooltip label={open.hostname} withArrow>
            <Anchor
              href={open.href}
              target="_blank"
              rel="noreferrer"
              size="sm"
              c="var(--db-link)"
              display="inline-flex"
              w="fit-content"
              style={{ alignItems: "center", gap: 6 }}
              onClick={(event) => event.stopPropagation()}
            >
              Open
              <IconExternalLink size={12} />
            </Anchor>
          </Tooltip>
        ) : (
          <Box h="1.5rem" />
        )}
      </Stack>
    </ConsolePaper>
  );
}

function CatalogAvatar({ name, icon }: { name: string; icon?: string }) {
  const letter = name.charAt(0).toUpperCase() || "?";
  return (
    <Avatar
      src={icon || undefined}
      alt=""
      size={40}
      radius="sm"
      color={colorFor(name)}
      variant="light"
    >
      {letter}
    </Avatar>
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
        <Tooltip key={st.name} label={`${st.name}: ${st.health || "unknown"}`} withArrow>
          <Group gap={6} wrap="nowrap">
            <Box className="db-stage-dot" bg={healthDotColor(st.health)} />
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

function groupDeployables(
  items: DeployableSummary[],
): { group: string; items: DeployableSummary[] }[] {
  const buckets = new Map<string, DeployableSummary[]>();
  for (const item of items) {
    const group = item.group?.trim() || "other";
    const list = buckets.get(group) ?? [];
    list.push(item);
    buckets.set(group, list);
  }
  const sections: { group: string; items: DeployableSummary[] }[] = [];
  for (const group of GROUP_ORDER) {
    const grouped = buckets.get(group);
    if (grouped) {
      sections.push({ group, items: grouped });
      buckets.delete(group);
    }
  }
  for (const group of [...buckets.keys()].sort()) {
    sections.push({ group, items: buckets.get(group) ?? [] });
  }
  return sections;
}

function groupLabel(group: string): string {
  if (!group) return "Other";
  return group.charAt(0).toUpperCase() + group.slice(1);
}

function colorFor(name: string): string {
  let h = 0;
  for (let i = 0; i < name.length; i++) {
    h = (h * 31 + name.charCodeAt(i)) >>> 0;
  }
  return AVATAR_COLORS[h % AVATAR_COLORS.length];
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
