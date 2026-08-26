import { Button, Stack, Text } from "@mantine/core";
import { Link, useOutletContext } from "react-router";

import type { DeployableContext } from "./deployables.$name";
import { CompactImage } from "~/ui/compact-image";
import { HostnameLink, StageObservabilityIcons } from "~/ui/external-links";
import { ReleaseFlow } from "~/ui/release-flow";
import { RelativeTime } from "~/ui/relative-time";
import { ResourceTable, Table } from "~/ui/resource-table";
import { stageStaleHint, StatusBadge } from "~/ui/status-badge";
import { StageReady } from "~/ui/workload-pods";

export default function DeployableOverview() {
  const { status, stages } = useOutletContext<DeployableContext>();

  return (
    <Stack gap="lg">
      <ReleaseFlow stages={stages} flow={status.flow} />

      <ResourceTable
        headers={[
          "Stage",
          "Hostname",
          "Image",
          "Sync",
          "Health",
          "Ready",
          "Deployed",
          "Links",
          "",
        ]}
        isEmpty={stages.length === 0}
        minWidth={880}
      >
        {stages.map((st) => {
          const stale = stageStaleHint(st);
          return (
            <Table.Tr key={st.name}>
              <Table.Td className="db-cell-fit" fw={600}>
                {st.name}
              </Table.Td>
              <Table.Td className="db-cell-fit">
                <HostnameLink hostname={st.hostname} />
              </Table.Td>
              <Table.Td className="db-cell-clip">
                <CompactImage value={st.image} empty="—" />
              </Table.Td>
              <Table.Td className="db-cell-fit">
                <StatusBadge status={st.sync} href={st.argoURL} />
              </Table.Td>
              <Table.Td>
                <StatusBadge status={st.health} href={st.argoURL} hint={stale} />
                {stale ? (
                  <Text size="xs" c="orange.4">
                    {stale}
                  </Text>
                ) : null}
                {st.message && st.connected !== false ? (
                  <Text size="xs" c="dimmed">
                    {st.message}
                  </Text>
                ) : null}
              </Table.Td>
              <Table.Td className="db-cell-fit">
                <StageReady workload={st.workload} />
              </Table.Td>
              <Table.Td className="db-cell-fit">
                <RelativeTime value={st.deployedAt} />
              </Table.Td>
              <Table.Td className="db-cell-fit">
                <StageObservabilityIcons
                  headlampURL={st.headlampURL}
                  grafanaURL={st.grafanaURL}
                  logsURL={st.logsURL}
                />
              </Table.Td>
              <Table.Td className="db-cell-fit">
                <Button
                  component={Link}
                  to={`/deployables/${status.name}/reconcile/${st.name}`}
                  variant="default"
                  size="compact-sm"
                >
                  Reconcile
                </Button>
              </Table.Td>
            </Table.Tr>
          );
        })}
      </ResourceTable>
    </Stack>
  );
}
