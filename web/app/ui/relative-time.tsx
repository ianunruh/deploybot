import { Text } from "@mantine/core";

import { formatAbsolute, formatRelative, parseDate } from "~/lib/time";

export function RelativeTime({
  value,
  size = "sm",
}: {
  value?: string | null;
  size?: "xs" | "sm" | "md";
}) {
  if (value == null || value === "") {
    return (
      <Text size={size} c="dimmed">
        —
      </Text>
    );
  }
  const d = parseDate(value);
  if (d == null) {
    return (
      <Text size={size} c="dimmed">
        —
      </Text>
    );
  }
  return (
    <Text size={size} c="dimmed" title={formatAbsolute(d)} span>
      {formatRelative(d)}
    </Text>
  );
}
