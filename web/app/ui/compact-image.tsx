import { Text } from "@mantine/core";

export function CompactImage({ value, empty = "—" }: { value?: string; empty?: string }) {
  if (!value) {
    return (
      <Text size="xs" c="dimmed" span>
        {empty}
      </Text>
    );
  }
  const at = value.indexOf("@");
  const tag = at >= 0 ? value.slice(0, at) : value;
  return (
    <Text className="db-clip-text" size="xs" ff="monospace" title={value} span>
      {tag}
    </Text>
  );
}

export function shortDigest(digest?: string): string {
  if (!digest) return "";
  const hex = digest.replace(/^sha256:/, "");
  return hex ? `sha256:${hex.slice(0, 12)}` : digest;
}
