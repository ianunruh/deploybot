import { Anchor, Badge, Text, Tooltip } from "@mantine/core";
import { Link } from "react-router";

import { formatRelative } from "~/lib/time";
import type { StageStatus } from "~/lib/api.server";

const STATUS_COLORS: Record<string, string> = {
  Healthy: "teal",
  Synced: "teal",
  Progressing: "yellow",
  OutOfSync: "orange",
  Degraded: "red",
  Missing: "red",
  Failed: "red",
  unknown: "gray",
  Unknown: "gray",
  Running: "teal",
  Pending: "yellow",
  Succeeded: "gray",
  Completed: "gray",
  CrashLoopBackOff: "red",
  Error: "red",
  ErrImagePull: "red",
  ImagePullBackOff: "red",
  OOMKilled: "red",
  ContainerCreating: "yellow",
  PodInitializing: "yellow",
  Terminating: "orange",
  NotReady: "yellow",
  success: "teal",
  failure: "red",
  "in progress": "yellow",
  queued: "yellow",
  pending: "yellow",
  waiting: "yellow",
  requested: "yellow",
  cancelled: "gray",
  skipped: "gray",
  completed: "gray",
  "timed out": "red",
  "action required": "orange",
  neutral: "gray",
  stale: "orange",
};

const KIND_COLORS: Record<string, string> = {
  pin: "teal",
  promote: "accent",
  rollback: "orange",
  overlay: "gray",
};

export function EventKindBadge({ kind }: { kind: string }) {
  return (
    <Badge
      color={KIND_COLORS[kind] ?? "gray"}
      variant="light"
      size="sm"
      radius="sm"
      tt="uppercase"
      styles={{
        root: {
          fontFamily: "inherit",
          letterSpacing: "0.04em",
          maxWidth: "none",
          overflow: "visible",
          flexShrink: 0,
        },
        label: {
          overflow: "visible",
          textOverflow: "unset",
          whiteSpace: "nowrap",
        },
      }}
    >
      {kind || "overlay"}
    </Badge>
  );
}

export function UpdateBadge({
  stale,
  error,
  to,
}: {
  stale?: boolean;
  error?: boolean;
  to?: string;
}) {
  const color = error ? "red" : stale ? "orange" : "teal";
  const label = error ? "error" : stale ? "behind" : "up-to-date";
  const badge = (
    <Badge
      color={color}
      variant="light"
      size="sm"
      radius="sm"
      tt="uppercase"
      styles={{
        root: {
          fontFamily: "inherit",
          letterSpacing: "0.04em",
          maxWidth: "none",
          overflow: "visible",
          flexShrink: 0,
          cursor: to ? "pointer" : undefined,
        },
        label: {
          overflow: "visible",
          textOverflow: "unset",
          whiteSpace: "nowrap",
        },
      }}
    >
      {label}
    </Badge>
  );
  if (!to) return badge;
  return (
    <Anchor
      component={Link}
      to={to}
      underline="never"
      display="inline-flex"
      onClick={(event) => event.stopPropagation()}
    >
      {badge}
    </Anchor>
  );
}

export function ReplicaReady({ ready, desired }: { ready?: number; desired?: number }) {
  if (ready == null || desired == null) {
    return (
      <Text size="sm" c="dimmed">
        —
      </Text>
    );
  }
  const color =
    desired === 0 ? "gray" : ready >= desired ? "teal" : ready === 0 ? "red" : "yellow";
  return (
    <Badge
      color={color}
      variant="light"
      size="sm"
      radius="sm"
      tt="uppercase"
      styles={{
        root: {
          fontFamily: "inherit",
          letterSpacing: "0.04em",
          maxWidth: "none",
          overflow: "visible",
          flexShrink: 0,
        },
        label: {
          overflow: "visible",
          textOverflow: "unset",
          whiteSpace: "nowrap",
        },
      }}
    >
      {ready}/{desired}
    </Badge>
  );
}

export function stageStaleHint(
  stage: Pick<StageStatus, "connected" | "updatedAt">,
): string | undefined {
  if (stage.connected !== false) return undefined;
  if (stage.updatedAt) return `last seen ${formatRelative(stage.updatedAt)}`;
  return "unreachable";
}

export function StatusBadge({
  status,
  href,
  hint,
}: {
  status: string;
  href?: string;
  hint?: string;
}) {
  const color = STATUS_COLORS[status] ?? (status.startsWith("Init:") ? "yellow" : "gray");
  const tooltip = hint ?? (href ? "Open in Argo CD" : undefined);
  const badge = (
    <Badge
      {...(href
        ? { component: "a" as const, href, target: "_blank", rel: "noreferrer" }
        : {})}
      color={color}
      variant="light"
      size="sm"
      radius="sm"
      tt="uppercase"
      styles={{
        root: {
          fontFamily: "inherit",
          letterSpacing: "0.04em",
          maxWidth: "none",
          overflow: "visible",
          flexShrink: 0,
          cursor: href ? "pointer" : undefined,
          textDecoration: "none",
        },
        label: {
          overflow: "visible",
          textOverflow: "unset",
          whiteSpace: "nowrap",
        },
      }}
    >
      {status || "Unknown"}
    </Badge>
  );
  if (!tooltip) return badge;
  return (
    <Tooltip label={tooltip} withArrow>
      {badge}
    </Tooltip>
  );
}
