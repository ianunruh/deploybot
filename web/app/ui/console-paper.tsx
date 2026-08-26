import { Paper, type PaperProps } from "@mantine/core";
import type { HTMLAttributes, ReactNode } from "react";

const consoleStyle = {
  background: "var(--db-panel)",
  border: "1px solid var(--db-border)",
  minWidth: 0,
  maxWidth: "100%",
} as const;

export function ConsolePaper({
  children,
  style,
  ...props
}: PaperProps & HTMLAttributes<HTMLDivElement> & { children: ReactNode }) {
  return (
    <Paper p="md" radius="sm" style={{ ...consoleStyle, ...style }} {...props}>
      {children}
    </Paper>
  );
}
