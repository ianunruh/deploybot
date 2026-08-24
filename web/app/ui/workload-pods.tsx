import { Group, Stack, Text } from "@mantine/core";

import type { StageStatus, WorkloadLive } from "~/lib/api.server";
import { RelativeTime } from "~/ui/relative-time";
import { ResourceTable, Table } from "~/ui/resource-table";
import { ReplicaReady, StatusBadge } from "~/ui/status-badge";

type PodRow = {
  stage: string;
  name: string;
  ready: string;
  status: string;
  restarts: number;
  ip?: string;
  node?: string;
  createdAt?: string;
};

export function WorkloadPods({ stages }: { stages: StageStatus[] }) {
  const live = stages.filter((st) => st.workload != null);
  if (live.length === 0) return null;

  const kind = live.find((st) => st.workload?.kind)?.workload?.kind;
  const name = live.find((st) => st.workload?.name)?.workload?.name;
  const errors = live.flatMap((st) =>
    st.workload?.message ? [`${st.name}: ${st.workload.message}`] : [],
  );
  const pods: PodRow[] = live.flatMap((st) =>
    (st.workload?.pods ?? []).map((p) => ({
      stage: st.name,
      name: p.name,
      ready: p.ready,
      status: p.status,
      restarts: p.restarts,
      ip: p.ip,
      node: p.node,
      createdAt: p.createdAt,
    })),
  );

  return (
    <Stack gap="sm">
      <Group justify="space-between" align="baseline">
        <Text size="sm" tt="uppercase" c="dimmed" fw={600}>
          Workload
        </Text>
        {kind || name ? (
          <Text size="xs" c="dimmed">
            {[kind, name].filter(Boolean).join(" · ")}
          </Text>
        ) : null}
      </Group>
      {errors.map((err) => (
        <Text key={err} size="xs" c="red.4">
          {err}
        </Text>
      ))}
      <ResourceTable
        headers={["Stage", "Name", "Ready", "Status", "Restarts", "Age", "IP", "Node"]}
        isEmpty={pods.length === 0}
        emptyMessage="No pods."
        minWidth={880}
      >
        {pods.map((p) => (
          <Table.Tr key={`${p.stage}:${p.name}`}>
            <Table.Td className="db-cell-fit" fw={600}>
              {p.stage}
            </Table.Td>
            <Table.Td className="db-cell-clip">
              <Text className="db-clip-text" size="sm" ff="monospace" title={p.name}>
                {p.name}
              </Text>
            </Table.Td>
            <Table.Td className="db-cell-fit">{p.ready || "—"}</Table.Td>
            <Table.Td className="db-cell-fit">
              <StatusBadge status={p.status} />
            </Table.Td>
            <Table.Td className="db-cell-fit">{p.restarts}</Table.Td>
            <Table.Td className="db-cell-fit">
              <RelativeTime value={p.createdAt} size="xs" />
            </Table.Td>
            <Table.Td className="db-cell-fit">{p.ip || "—"}</Table.Td>
            <Table.Td className="db-cell-fit">{p.node || "—"}</Table.Td>
          </Table.Tr>
        ))}
      </ResourceTable>
    </Stack>
  );
}

export function StageReady({ workload }: { workload?: WorkloadLive }) {
  if (
    workload == null ||
    (workload.message != null &&
      workload.message !== "" &&
      workload.desired === 0 &&
      (workload.pods == null || workload.pods.length === 0))
  ) {
    return (
      <Text size="sm" c="dimmed">
        —
      </Text>
    );
  }
  return <ReplicaReady ready={workload.ready} desired={workload.desired} />;
}
