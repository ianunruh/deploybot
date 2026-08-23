import { Table, Text } from "@mantine/core";
import type { ReactNode } from "react";

const DEFAULT_MIN_WIDTH = 720;

export function ResourceTable({
  headers,
  children,
  emptyMessage = "No resources found.",
  isEmpty,
  minWidth = DEFAULT_MIN_WIDTH,
}: {
  headers: ReactNode[];
  children: ReactNode;
  emptyMessage?: string;
  isEmpty?: boolean;
  minWidth?: number | string;
}) {
  if (isEmpty) {
    return (
      <Text c="dimmed" size="sm" py="xl" ta="center">
        {emptyMessage}
      </Text>
    );
  }

  return (
    <Table.ScrollContainer className="db-table-scroll" minWidth={minWidth} type="native">
      <Table
        className="db-table"
        highlightOnHover
        verticalSpacing="sm"
        horizontalSpacing="md"
        withRowBorders
      >
        <Table.Thead>
          <Table.Tr>
            {headers.map((header, i) => (
              <Table.Th key={i}>{header}</Table.Th>
            ))}
          </Table.Tr>
        </Table.Thead>
        <Table.Tbody>{children}</Table.Tbody>
      </Table>
    </Table.ScrollContainer>
  );
}

export { Table };
