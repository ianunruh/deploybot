import { Box, Code, Group, Text } from "@mantine/core";
import { ConsolePaper } from "./console-paper";

export function DiffPanel({ diff, title = "Diff" }: { diff: string; title?: string }) {
  return (
    <ConsolePaper>
      <Group justify="space-between" mb="sm">
        <Text size="xs" c="dimmed" tt="uppercase" fw={600}>
          {title}
        </Text>
      </Group>
      {diff ? (
        <Box
          className="db-yaml"
          style={{
            maxHeight: 360,
            overflow: "auto",
            borderRadius: 4,
            border: "1px solid #1e242c",
          }}
        >
          <Code block style={{ background: "transparent", color: "inherit" }}>
            {diff}
          </Code>
        </Box>
      ) : (
        <Text size="sm" c="dimmed">
          No changes.
        </Text>
      )}
    </ConsolePaper>
  );
}
