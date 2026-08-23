import { Alert, Stack, Text } from "@mantine/core";
import { Link } from "react-router";

import type { Route } from "./+types/home";
import { listDeployables } from "~/lib/api.server";
import { PageHeader } from "~/ui/page-header";
import { ResourceTable, Table } from "~/ui/resource-table";

export function meta(_args: Route.MetaArgs) {
  return [{ title: "deploybot" }];
}

export async function loader(_args: Route.LoaderArgs) {
  try {
    const data = await listDeployables();
    return { deployables: data.deployables, error: null as string | null };
  } catch (err) {
    return {
      deployables: [],
      error: err instanceof Error ? err.message : String(err),
    };
  }
}

export default function Home({ loaderData }: Route.ComponentProps) {
  const { deployables, error } = loaderData;

  return (
    <Stack gap="lg">
      <PageHeader
        title="Deployables"
        description="Catalog of apps deploybot can pin and promote."
      />
      {error != null && (
        <Alert color="red" title="API unavailable">
          {error}. Start the Go API with `just serve`.
        </Alert>
      )}
      <ResourceTable
        headers={["Name", "Namespace", "Image", "Stages"]}
        isEmpty={deployables.length === 0 && error == null}
        emptyMessage="No deployable specs found."
      >
        {deployables.map((d) => (
          <Table.Tr key={d.name}>
            <Table.Td>
              <Text component={Link} to={`/deployables/${d.name}`} fw={600} c="accent.4">
                {d.name}
              </Text>
            </Table.Td>
            <Table.Td>{d.namespace}</Table.Td>
            <Table.Td>
              <Text size="sm" ff="monospace">
                {d.image}
              </Text>
            </Table.Td>
            <Table.Td>{d.stages.join(" → ")}</Table.Td>
          </Table.Tr>
        ))}
      </ResourceTable>
    </Stack>
  );
}
