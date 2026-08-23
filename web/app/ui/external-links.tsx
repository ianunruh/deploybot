import { ActionIcon, Anchor, Group, Text, Tooltip } from "@mantine/core";
import {
  IconBrandGit,
  IconBrandGithub,
  IconBrandGitlab,
  IconBrandTrello,
  IconExternalLink,
  IconLayoutKanban,
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
    <Anchor href={href} target="_blank" rel="noreferrer" size="sm" c="accent.4">
      <Group gap={6} wrap="nowrap">
        <Glyph size={14} />
        {label}
      </Group>
    </Anchor>
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
      c="accent.4"
    >
      {hostname}
    </Anchor>
  );
}

export function ArgoCDLink({ href }: { href?: string }) {
  if (!href) return null;
  return <LinkIcon href={href} label="Argo CD" icon={IconExternalLink} />;
}
