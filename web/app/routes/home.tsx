import { Alert, Group, Stack, Text } from "@mantine/core";
import { Link } from "react-router";

import type { Route } from "./+types/home";
import { listDeployables } from "~/lib/api.server";
import { DeployableLinkIcons, ObservabilityClusterMenus } from "~/ui/external-links";
import { PageHeader } from "~/ui/page-header";
import { RelativeTime } from "~/ui/relative-time";
import { ReleaseFlowInline } from "~/ui/release-flow";
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
        description="Catalog of apps deploybot can pin, promote, and reconcile."
      />
      {error != null && (
        <Alert color="red" title="API unavailable">
          {error}. Start the Go API with `just serve`.
        </Alert>
      )}
      <ResourceTable
        headers={["Name", "Namespace", "Flow", "Last deploy", "Links"]}
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
              <ReleaseFlowInline stages={d.stages ?? []} flow={d.flow} />
            </Table.Td>
            <Table.Td className="db-cell-fit">
              <RelativeTime value={d.deployedAt} />
            </Table.Td>
            <Table.Td>
              {d.repoURL ||
              d.projectURL ||
              (d.stages ?? []).some(
                (st) => st.headlampURL || st.grafanaURL || st.logsURL,
              ) ? (
                <Group gap={2} wrap="nowrap">
                  <DeployableLinkIcons repoURL={d.repoURL} projectURL={d.projectURL} />
                  <ObservabilityClusterMenus stages={d.stages ?? []} />
                </Group>
              ) : (
                <Text size="sm" c="dimmed">
                  —
                </Text>
              )}
            </Table.Td>
          </Table.Tr>
        ))}
      </ResourceTable>
    </Stack>
  );
}
