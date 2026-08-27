import { Alert, Button, Stack, Text } from "@mantine/core";
import { useDisclosure } from "@mantine/hooks";
import { useEffect, useState } from "react";
import { Link, useFetcher, useRevalidator } from "react-router";

import type { Route } from "./+types/updates";
import {
  actorHeaders,
  listUpdates,
  pinDeployable,
  type MutationResult,
  type UpdateStatus,
} from "~/lib/api.server";
import { notifyActionError, notifyActionSuccess } from "~/lib/action-feedback";
import { useFetcherResult } from "~/lib/use-fetcher-result";
import { CompactImage } from "~/ui/compact-image";
import { ConfirmActionModal } from "~/ui/confirm-action-modal";
import { DiffPanel } from "~/ui/diff-panel";
import {
  ArgoSyncCheckbox,
  formFlag,
  MutationGitHint,
  mutationCommitLabel,
  mutationNote,
} from "~/ui/mutation-controls";
import { PageHeader } from "~/ui/page-header";
import { RelativeTime } from "~/ui/relative-time";
import {
  matchesResourceFilters,
  ResourceFilterBar,
  uniqueSorted,
  useResourceFilters,
} from "~/ui/resource-filter";
import { ResourceTable, Table } from "~/ui/resource-table";
import { UpdateBadge } from "~/ui/status-badge";

export function meta(_args: Route.MetaArgs) {
  return [{ title: "Updates · deploybot" }];
}

export async function loader(_args: Route.LoaderArgs) {
  try {
    const data = await listUpdates();
    return { ...data, error: null as string | null };
  } catch (err) {
    return {
      updates: [] as UpdateStatus[],
      apply: false,
      push: false,
      sync: false,
      error: err instanceof Error ? err.message : String(err),
    };
  }
}

type ActionData =
  { ok: true; intent: string; result?: MutationResult } | { ok: false; error: string };

export async function action({ request }: Route.ActionArgs) {
  const form = await request.formData();
  const intent = String(form.get("intent") ?? "");
  try {
    if (intent !== "pin") {
      return { ok: false, error: `unknown intent ${intent}` } satisfies ActionData;
    }
    return {
      ok: true,
      intent,
      result: await pinDeployable(
        String(form.get("name") ?? ""),
        String(form.get("stage") ?? ""),
        String(form.get("image") ?? ""),
        {
          sync: formFlag(form, "sync"),
          wait: formFlag(form, "wait"),
          headers: actorHeaders(request),
        },
      ),
    } satisfies ActionData;
  } catch (err) {
    return {
      ok: false,
      error: err instanceof Error ? err.message : String(err),
    } satisfies ActionData;
  }
}

export default function Updates({ loaderData }: Route.ComponentProps) {
  const { updates, apply, push, sync, error } = loaderData;
  const revalidator = useRevalidator();
  const pinFetcher = useFetcher<ActionData>();
  const [pinOpen, pinHandlers] = useDisclosure(false);
  const [selected, setSelected] = useState<UpdateStatus | null>(null);
  const [pinSync, setPinSync] = useState(true);
  const [filters, setFilters] = useResourceFilters();
  const filtered = updates.filter((u) => matchesResourceFilters(u, filters));

  useEffect(() => {
    const id = window.setInterval(() => {
      if (revalidator.state === "idle") void revalidator.revalidate();
    }, 30_000);
    return () => window.clearInterval(id);
  }, [revalidator]);

  useFetcherResult(pinFetcher, (data) => {
    if (!data.ok) {
      notifyActionError("Pin failed", data.error);
      return;
    }
    notifyActionSuccess(
      "Pin",
      `Wrote overlay${mutationNote(data.result, { argoAvailable: sync })}`,
    );
    pinHandlers.close();
    setSelected(null);
    void revalidator.revalidate();
  });

  return (
    <Stack gap="lg">
      <PageHeader
        title="Updates"
        description="Upstream images tracked with spec.update. Auto-enrolled apps pin the first stage on a schedule; promote gates still apply."
      />
      {error != null && (
        <Alert color="red" title="API unavailable">
          {error}. Start the Go API with `just serve`.
        </Alert>
      )}
      {updates.length > 0 ? (
        <ResourceFilterBar
          value={filters}
          onChange={setFilters}
          namespaces={uniqueSorted(updates.map((u) => u.namespace))}
          projects={uniqueSorted(updates.map((u) => u.project))}
        />
      ) : null}
      <ResourceTable
        headers={["Name", "Status", "Current", "Newest", "Auto", "Checked", ""]}
        isEmpty={filtered.length === 0 && error == null}
        emptyMessage={
          updates.length === 0
            ? "No deployables opt into registry tracking (spec.update)."
            : "No updates match these filters."
        }
        minWidth={960}
      >
        {filtered.map((u) => (
          <Table.Tr key={u.name}>
            <Table.Td>
              <Text
                component={Link}
                to={`/deployables/${u.name}`}
                fw={600}
                c="var(--db-link)"
              >
                {u.name}
              </Text>
            </Table.Td>
            <Table.Td className="db-cell-fit">
              <UpdateBadge stale={u.stale} error={Boolean(u.error)} />
            </Table.Td>
            <Table.Td className="db-cell-clip">
              <CompactImage value={u.current.compact} empty="—" />
            </Table.Td>
            <Table.Td>
              <Stack gap={2}>
                <CompactImage value={u.newest?.tag ?? u.newest?.ref} empty="—" />
                {u.newest?.createdAt ? (
                  <Text size="xs" c="dimmed">
                    <RelativeTime value={u.newest.createdAt} />
                  </Text>
                ) : null}
              </Stack>
            </Table.Td>
            <Table.Td className="db-cell-fit">
              <Text size="sm" c="dimmed">
                {u.auto ?? "manual"}
              </Text>
            </Table.Td>
            <Table.Td className="db-cell-fit">
              {u.error ? (
                <Text size="xs" c="red" title={u.error}>
                  {u.error}
                </Text>
              ) : (
                <RelativeTime value={u.checkedAt} />
              )}
            </Table.Td>
            <Table.Td className="db-cell-fit">
              <Button
                variant="default"
                size="compact-sm"
                disabled={!u.stale || !u.newest?.ref}
                onClick={() => {
                  setSelected(u);
                  setPinSync(true);
                  pinHandlers.open();
                }}
              >
                Pin newest
              </Button>
            </Table.Td>
          </Table.Tr>
        ))}
      </ResourceTable>

      {pinFetcher.data?.ok ? (
        <DiffPanel diff={pinFetcher.data.result?.diff ?? ""} title="Last mutation diff" />
      ) : null}

      <ConfirmActionModal
        opened={pinOpen}
        onClose={() => {
          pinHandlers.close();
          setSelected(null);
        }}
        loading={pinFetcher.state !== "idle"}
        title={selected ? `Pin ${selected.name}` : "Pin newest"}
        confirmLabel={mutationCommitLabel({ apply, push }, "pin", sync && pinSync)}
        confirmDisabled={selected == null || !selected.newest?.ref}
        message={
          selected ? (
            <Stack gap="sm">
              <Text size="sm">
                Pin <strong>{selected.stage}</strong> to{" "}
                <CompactImage
                  value={selected.newest?.tag ?? selected.newest?.ref}
                  empty="—"
                />{" "}
                (currently{" "}
                <CompactImage value={selected.current.compact} empty="unpinned" />
                ). Later stages still follow promote gates.
              </Text>
              <ArgoSyncCheckbox
                show={apply && sync}
                checked={pinSync}
                onChange={setPinSync}
                stage={selected.stage}
              />
              <MutationGitHint
                apply={apply}
                push={push}
                sync={sync && pinSync}
                syncStage={selected.stage}
              />
            </Stack>
          ) : (
            <Text size="sm">Select an image.</Text>
          )
        }
        onConfirm={() => {
          if (!selected?.newest?.ref) return;
          void pinFetcher.submit(
            {
              intent: "pin",
              name: selected.name,
              stage: selected.stage,
              image: selected.newest.ref,
              ...(sync ? { sync: pinSync ? "true" : "false" } : {}),
              ...(apply ? { wait: "false" } : {}),
            },
            { method: "post" },
          );
        }}
      />
    </Stack>
  );
}
