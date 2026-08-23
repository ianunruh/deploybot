import { Badge } from "@mantine/core";

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
};

export function StatusBadge({ status }: { status: string }) {
  const color = STATUS_COLORS[status] ?? "gray";
  return (
    <Badge
      color={color}
      variant="light"
      size="sm"
      radius="sm"
      tt="uppercase"
      style={{ fontFamily: "inherit", letterSpacing: "0.04em" }}
    >
      {status || "Unknown"}
    </Badge>
  );
}
