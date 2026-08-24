import { ActionIcon, Anchor, Group, Menu, Text, Tooltip } from "@mantine/core";
import {
  IconBrandGit,
  IconBrandGithub,
  IconBrandGitlab,
  IconBrandTrello,
  IconChartHistogram,
  IconLayoutKanban,
  IconLogs,
  IconSitemap,
  type Icon,
} from "@tabler/icons-react";

function hostOf(href: string): string {
  try {
    return new URL(href).hostname.replace(/^www\./, "").toLowerCase();
  } catch {
    return "";
  }
}

function repoIcon(href: string): Icon {
  const host = hostOf(href);
  if (host === "github.com" || host.endsWith(".github.com")) return IconBrandGithub;
  if (host === "gitlab.com" || host.includes("gitlab")) return IconBrandGitlab;
  return IconBrandGit;
}

function projectIcon(href: string): Icon {
  const host = hostOf(href);
  if (host === "trello.com" || host.endsWith(".trello.com")) return IconBrandTrello;
  return IconLayoutKanban;
}

function LinkIcon({
  href,
  label,
  icon: Glyph,
  size = 16,
}: {
  href: string;
  label: string;
  icon: Icon;
  size?: number;
}) {
  return (
    <Tooltip label={label} withArrow>
      <ActionIcon
        component="a"
        href={href}
        target="_blank"
        rel="noreferrer"
        variant="subtle"
        color="gray"
        size="sm"
        aria-label={label}
      >
        <Glyph size={size} />
      </ActionIcon>
    </Tooltip>
  );
}

export function DeployableLinkIcons({
  repoURL,
  projectURL,
  size = 16,
}: {
  repoURL?: string;
  projectURL?: string;
  size?: number;
}) {
  if (!repoURL && !projectURL) return null;
  return (
    <Group gap={2} wrap="nowrap">
      {repoURL ? (
        <LinkIcon href={repoURL} label="Repo" icon={repoIcon(repoURL)} size={size} />
      ) : null}
      {projectURL ? (
        <LinkIcon
          href={projectURL}
          label="Project"
          icon={projectIcon(projectURL)}
          size={size}
        />
      ) : null}
    </Group>
  );
}

export function DeployableLinkLabels({
  repoURL,
  projectURL,
}: {
  repoURL?: string;
  projectURL?: string;
}) {
  if (!repoURL && !projectURL) return null;
  return (
    <Group gap="md" wrap="wrap">
      {repoURL ? (
        <LabeledLink href={repoURL} label="Repo" icon={repoIcon(repoURL)} />
      ) : null}
      {projectURL ? (
        <LabeledLink href={projectURL} label="Project" icon={projectIcon(projectURL)} />
      ) : null}
    </Group>
  );
}

function LabeledLink({
  href,
  label,
  icon: Glyph,
}: {
  href: string;
  label: string;
  icon: Icon;
}) {
  return (
    <Anchor href={href} target="_blank" rel="noreferrer" size="sm" c="var(--db-link)">
      <Group gap={6} wrap="nowrap">
        <Glyph size={14} />
        {label}
      </Group>
    </Anchor>
  );
}

type StageObsLinks = {
  name: string;
  headlampURL?: string;
  grafanaURL?: string;
  logsURL?: string;
};

type ClusterLink = { cluster: string; href: string };

function clusterLinks(
  stages: StageObsLinks[],
  key: "headlampURL" | "grafanaURL" | "logsURL",
): ClusterLink[] {
  return stages.flatMap((st) => {
    const href = st[key];
    return href ? [{ cluster: st.name, href }] : [];
  });
}

function ClusterMenuIcon({
  label,
  icon: Glyph,
  links,
  size = 16,
}: {
  label: string;
  icon: Icon;
  links: ClusterLink[];
  size?: number;
}) {
  if (links.length === 0) return null;
  return (
    <Menu position="bottom-end" withinPortal shadow="md">
      <Menu.Target>
        <Tooltip label={label} withArrow>
          <ActionIcon variant="subtle" color="gray" size="sm" aria-label={label}>
            <Glyph size={size} />
          </ActionIcon>
        </Tooltip>
      </Menu.Target>
      <Menu.Dropdown>
        {links.map((link) => (
          <Menu.Item
            key={link.cluster}
            component="a"
            href={link.href}
            target="_blank"
            rel="noreferrer"
          >
            {link.cluster}
          </Menu.Item>
        ))}
      </Menu.Dropdown>
    </Menu>
  );
}

export function ObservabilityClusterMenus({
  stages,
  size = 16,
}: {
  stages: StageObsLinks[];
  size?: number;
}) {
  const headlamp = clusterLinks(stages, "headlampURL");
  const grafana = clusterLinks(stages, "grafanaURL");
  const logs = clusterLinks(stages, "logsURL");
  if (headlamp.length === 0 && grafana.length === 0 && logs.length === 0) return null;
  return (
    <Group gap={2} wrap="nowrap">
      <ClusterMenuIcon label="Headlamp" icon={IconSitemap} links={headlamp} size={size} />
      <ClusterMenuIcon
        label="Resources"
        icon={IconChartHistogram}
        links={grafana}
        size={size}
      />
      <ClusterMenuIcon label="Logs" icon={IconLogs} links={logs} size={size} />
    </Group>
  );
}

export function StageObservabilityIcons({
  headlampURL,
  grafanaURL,
  logsURL,
  stage,
  size = 16,
}: {
  headlampURL?: string;
  grafanaURL?: string;
  logsURL?: string;
  stage?: string;
  size?: number;
}) {
  if (!headlampURL && !grafanaURL && !logsURL) return null;
  const suffix = stage ? ` · ${stage}` : "";
  return (
    <Group gap={2} wrap="nowrap">
      {headlampURL ? (
        <LinkIcon
          href={headlampURL}
          label={`Headlamp${suffix}`}
          icon={IconSitemap}
          size={size}
        />
      ) : null}
      {grafanaURL ? (
        <LinkIcon
          href={grafanaURL}
          label={`Resources${suffix}`}
          icon={IconChartHistogram}
          size={size}
        />
      ) : null}
      {logsURL ? (
        <LinkIcon href={logsURL} label={`Logs${suffix}`} icon={IconLogs} size={size} />
      ) : null}
    </Group>
  );
}

export function HostnameLink({ hostname }: { hostname?: string }) {
  if (!hostname) {
    return (
      <Text size="sm" c="dimmed">
        —
      </Text>
    );
  }
  const href = /^https?:\/\//i.test(hostname) ? hostname : `https://${hostname}`;
  return (
    <Anchor
      href={href}
      target="_blank"
      rel="noreferrer"
      size="sm"
      ff="monospace"
      c="var(--db-link)"
    >
      {hostname}
    </Anchor>
  );
}
