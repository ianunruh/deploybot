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
      {status || "Unknown"}
    </Badge>
  );
}
