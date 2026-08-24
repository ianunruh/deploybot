import { Badge, Text, Tooltip } from "@mantine/core";

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

export function UpdateBadge({ stale, error }: { stale?: boolean; error?: boolean }) {
  const color = error ? "red" : stale ? "orange" : "teal";
  const label = error ? "error" : stale ? "behind" : "up-to-date";
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
      {label}
    </Badge>
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

export function StatusBadge({ status, href }: { status: string; href?: string }) {
  const color = STATUS_COLORS[status] ?? (status.startsWith("Init:") ? "yellow" : "gray");
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
  if (!href) return badge;
  return (
    <Tooltip label="Open in Argo CD" withArrow>
      {badge}
    </Tooltip>
  );
}
