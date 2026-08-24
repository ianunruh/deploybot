import {
  Cascader,
  createTheme,
  MultiSelect,
  Select,
  type MantineColorsTuple,
} from "@mantine/core";

const accent: MantineColorsTuple = [
  "#edf2ff",
  "#dbe4ff",
  "#bac8ff",
  "#91a7ff",
  "#748ffc",
  "#5c7cfa",
  "#4c6ef5",
  "#4263eb",
  "#3b5bdb",
  "#364fc7",
];

export const theme = createTheme({
  primaryColor: "accent",
  colors: {
    accent,
  },
  fontFamily:
    '"Geist Mono", ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace',
  fontFamilyMonospace:
    '"Geist Mono", ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace',
  defaultRadius: "sm",
  headings: {
    fontFamily:
      '"Geist Mono", ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace',
    fontWeight: "600",
  },
  other: {
    consoleBg: "var(--db-bg)",
    consolePanel: "var(--db-panel)",
    consoleBorder: "var(--db-border)",
  },
  components: {
    Select: Select.extend({
      defaultProps: { checkIconPosition: "right" },
    }),
    MultiSelect: MultiSelect.extend({
      defaultProps: { checkIconPosition: "right" },
    }),
    Cascader: Cascader.extend({
      defaultProps: { checkIconPosition: "right" },
    }),
  },
});
