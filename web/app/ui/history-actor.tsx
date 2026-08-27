import { Badge, Group, Text } from "@mantine/core";

import type { Actor } from "~/lib/api.server";

const ACTOR_COLORS: Record<string, string> = {
  "auto-pin": "gray",
  "auto-promote": "gray",
  "github-actions": "accent",
  user: "teal",
};

const ACTOR_LABELS: Record<string, string> = {
  "github-actions": "actions",
};

function actorKindLabel(kind: string): string {
  return ACTOR_LABELS[kind] ?? kind;
}

function actorIdentity(actor?: Actor): string {
  if (actor == null || !actor.kind) {
    return "";
  }
  if (actor.kind === "github-actions") {
    return actor.repo || actor.id || "";
  }
  if (actor.kind === "user") {
    return actor.id || actor.name || "";
  }
  return actor.id || "";
}

export function ActorKindBadge({
  kind,
  size = "sm",
}: {
  kind: string;
  size?: "sm" | "xs";
}) {
  return (
    <Badge
      color={ACTOR_COLORS[kind] ?? "gray"}
      variant="light"
      size={size}
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
      {actorKindLabel(kind) || "unknown"}
    </Badge>
  );
}

export function HistoryActor({
  actor,
  author,
  size = "sm",
}: {
  actor?: Actor;
  author?: string;
  size?: "sm" | "xs";
}) {
  const kind = actor?.kind;
  const identity = actorIdentity(actor);
  if (kind) {
    return (
      <Group gap={6} wrap="nowrap">
        <ActorKindBadge kind={kind} size={size} />
        {identity ? (
          <Text size={size} lineClamp={1} title={identity}>
            {identity}
          </Text>
        ) : null}
      </Group>
    );
  }
  return (
    <Text size={size} c="dimmed" lineClamp={1} title={author}>
      {author || "—"}
    </Text>
  );
}
