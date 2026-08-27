import {
  Alert,
  Button,
  Group,
  Select,
  Stack,
  Switch,
  Text,
  TextInput,
} from "@mantine/core";
import { useDisclosure } from "@mantine/hooks";
import { useEffect, useMemo, useState } from "react";
import { Link, useFetcher, useNavigate, useRevalidator } from "react-router";

import type { Route } from "./+types/ops";
import { notifyActionError, notifyActionSuccess } from "~/lib/action-feedback";
import {
  actorHeaders,
  getOpsCatalog,
  listOpsExecutions,
  startOpsExecution,
  type OpsCatalog,
  type OpsExecution,
  type OpsKind,
} from "~/lib/api.server";
import { useFetcherResult } from "~/lib/use-fetcher-result";
import { ConfirmActionModal } from "~/ui/confirm-action-modal";
import { ConsolePaper } from "~/ui/console-paper";
import { HistoryActor } from "~/ui/history-actor";
import { PageHeader } from "~/ui/page-header";
import { RelativeTime } from "~/ui/relative-time";
import { ResourceTable, Table } from "~/ui/resource-table";
import { StatusBadge } from "~/ui/status-badge";
import {
  cleanParams,
  OpsKindFields,
  requiredParamsFilled,
  type OpsParamValues,
} from "~/ui/ops-fields";

export function meta(_args: Route.MetaArgs) {
  return [{ title: "Ops · deploybot" }];
}

export async function loader(_args: Route.LoaderArgs) {
  try {
    const [catalog, listed] = await Promise.all([getOpsCatalog(), listOpsExecutions()]);
    return {
      catalog,
      executions: listed.executions,
      error: null as string | null,
    };
  } catch (err) {
    return {
      catalog: { kinds: [], clusters: [], imageSet: false } satisfies OpsCatalog,
      executions: [] as OpsExecution[],
      error: err instanceof Error ? err.message : String(err),
    };
  }
}

type ActionData = { ok: true; execution: OpsExecution } | { ok: false; error: string };

export async function action({ request }: Route.ActionArgs) {
  const form = await request.formData();
  if (String(form.get("intent") ?? "") !== "run") {
    return { ok: false, error: "unknown intent" } satisfies ActionData;
  }
  let params: unknown = {};
  const raw = String(form.get("params") ?? "");
  if (raw) {
    try {
      params = JSON.parse(raw) as unknown;
    } catch {
      return { ok: false, error: "invalid params JSON" } satisfies ActionData;
    }
  }
  try {
    const execution = await startOpsExecution(
      {
        kind: String(form.get("kind") ?? ""),
        cluster: String(form.get("cluster") ?? ""),
        dryRun: String(form.get("dryRun") ?? "true") !== "false",
        ref: String(form.get("ref") ?? "").trim() || undefined,
        params,
      },
      { headers: actorHeaders(request) },
    );
    return { ok: true, execution } satisfies ActionData;
  } catch (err) {
    return {
      ok: false,
      error: err instanceof Error ? err.message : String(err),
    } satisfies ActionData;
  }
}

export default function Ops({ loaderData }: Route.ComponentProps) {
  const { catalog, executions, error } = loaderData;
  const navigate = useNavigate();
  const revalidator = useRevalidator();
  const fetcher = useFetcher<ActionData>();
  const [confirmOpen, confirmHandlers] = useDisclosure(false);
  const kinds = catalog.kinds;
  const [kindName, setKindName] = useState(kinds[0]?.name ?? "");
  const kind: OpsKind | undefined = useMemo(
    () => kinds.find((k) => k.name === kindName) ?? kinds[0],
    [kinds, kindName],
  );
  const [cluster, setCluster] = useState(catalog.clusters[0] ?? "");
  const [dryRun, setDryRun] = useState(true);
  const [ref, setRef] = useState("");
  const [params, setParams] = useState<OpsParamValues>({});

  useEffect(() => {
    const id = window.setInterval(() => {
      if (revalidator.state === "idle") void revalidator.revalidate();
    }, 8_000);
    return () => window.clearInterval(id);
  }, [revalidator]);

  useFetcherResult(fetcher, (data) => {
    if (!data.ok) {
      notifyActionError("Ops run failed", data.error);
      return;
    }
    notifyActionSuccess("Ops", `Started ${data.execution.id}`);
    confirmHandlers.close();
    void navigate(`/ops/${data.execution.cluster}/${data.execution.id}`);
  });

  const payload = cleanParams(params);

  const submit = (apply: boolean) => {
    const form = new FormData();
    form.set("intent", "run");
    form.set("kind", kind?.name ?? "");
    form.set("cluster", cluster);
    form.set("dryRun", apply ? "false" : "true");
    if (ref.trim()) form.set("ref", ref.trim());
    form.set("params", JSON.stringify(payload));
    fetcher.submit(form, { method: "post" });
  };

  return (
    <Stack gap="lg">
      <PageHeader
        title="Ops"
        description="Run allowlisted kcloud-ops commands as Jobs on the target cluster. History is the Job list."
      />
      {error != null && (
        <Alert color="red" title="API unavailable">
          {error}. Start the Go API with `just serve`.
        </Alert>
      )}
      {!catalog.imageSet ? (
        <Alert color="yellow" title="Ops image not pinned">
          Set ops.image in deploybot.yaml (digest-pinned ghcr.io/ianunruh/kcloud-ops) to
          create Jobs. Catalog and history still load.
        </Alert>
      ) : null}

      <ConsolePaper>
        <Stack gap="md">
          <Text size="sm" fw={600}>
            New execution
          </Text>
          <Group grow>
            <Select
              label="Kind"
              data={kinds.map((k) => ({ value: k.name, label: k.title || k.name }))}
              value={kind?.name ?? null}
              onChange={(v) => {
                setKindName(v ?? "");
                setParams({});
              }}
            />
            <Select
              label="Cluster"
              data={catalog.clusters}
              value={cluster || null}
              onChange={(v) => setCluster(v ?? "")}
            />
          </Group>
          {kind ? (
            <OpsKindFields fields={kind.fields} values={params} onChange={setParams} />
          ) : null}
          <Group grow>
            <TextInput
              label="Git ref"
              placeholder={catalog.defaultRef || "main"}
              value={ref}
              onChange={(e) => setRef(e.currentTarget.value)}
            />
            <Switch
              label="Dry-run"
              description="Unchecked runs for real."
              checked={dryRun}
              onChange={(e) => setDryRun(e.currentTarget.checked)}
              mt="xl"
            />
          </Group>
          <Group>
            <Button
              loading={fetcher.state !== "idle"}
              disabled={
                !kind ||
                !cluster ||
                !catalog.imageSet ||
                !requiredParamsFilled(kind.fields, params)
              }
              onClick={() => {
                if (dryRun) submit(false);
                else confirmHandlers.open();
              }}
            >
              {dryRun ? "Run dry-run" : "Run"}
            </Button>
          </Group>
        </Stack>
      </ConsolePaper>

      <ResourceTable
        headers={["Status", "Kind", "Cluster", "Summary", "Dry-run", "Actor", "Age"]}
        isEmpty={executions.length === 0 && error == null}
        emptyMessage="No executions yet."
        minWidth={960}
      >
        {executions.map((ex) => (
          <Table.Tr key={`${ex.cluster}:${ex.id}`}>
            <Table.Td className="db-cell-fit">
              <StatusBadge status={ex.phase} />
            </Table.Td>
            <Table.Td className="db-cell-fit">{ex.kind}</Table.Td>
            <Table.Td className="db-cell-fit">{ex.cluster}</Table.Td>
            <Table.Td className="db-cell-clip">
              <Text
                component={Link}
                to={`/ops/${ex.cluster}/${ex.id}`}
                className="db-clip-text"
                size="sm"
                c="var(--db-link)"
                title={ex.summary}
              >
                {ex.summary || ex.id}
              </Text>
            </Table.Td>
            <Table.Td className="db-cell-fit">{ex.dryRun ? "yes" : "no"}</Table.Td>
            <Table.Td className="db-cell-fit">
              <HistoryActor actor={ex.actor} />
            </Table.Td>
            <Table.Td className="db-cell-fit">
              <RelativeTime value={ex.createdAt} size="xs" />
            </Table.Td>
          </Table.Tr>
        ))}
      </ResourceTable>

      <ConfirmActionModal
        opened={confirmOpen}
        onClose={confirmHandlers.close}
        loading={fetcher.state !== "idle"}
        title="Run for real"
        confirmLabel="Run"
        confirmColor="red"
        message={
          <Text size="sm">
            Start <strong>{kind?.title ?? kindName}</strong> on <strong>{cluster}</strong>
            {Object.keys(payload).length > 0 ? ` (${JSON.stringify(payload)})` : ""}. This
            is not a dry-run.
          </Text>
        }
        onConfirm={() => submit(true)}
      />
    </Stack>
  );
}
