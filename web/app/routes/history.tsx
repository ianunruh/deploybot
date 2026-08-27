import { Alert, Anchor, Stack, Text } from "@mantine/core";
import { Link } from "react-router";

import type { Route } from "./+types/history";
import { listHistory, type HistoryEvent } from "~/lib/api.server";
import { CompactImage } from "~/ui/compact-image";
import { HistoryActor } from "~/ui/history-actor";
import { PageHeader } from "~/ui/page-header";
import { RelativeTime } from "~/ui/relative-time";
import {
  matchesResourceFilters,
  ResourceFilterBar,
  uniqueSorted,
  useResourceFilters,
} from "~/ui/resource-filter";
import { ResourceTable, Table } from "~/ui/resource-table";
import { EventKindBadge } from "~/ui/status-badge";

export function meta(_args: Route.MetaArgs) {
  return [{ title: "History · deploybot" }];
}

export async function loader(_args: Route.LoaderArgs) {
  try {
    const data = await listHistory();
    return { events: data.events, error: null as string | null };
  } catch (err) {
    return {
      events: [] as HistoryEvent[],
      error: err instanceof Error ? err.message : String(err),
    };
  }
}

export default function History({ loaderData }: Route.ComponentProps) {
  const { events, error } = loaderData;
  const [filters, setFilters] = useResourceFilters();
  const filtered = events.filter((event) =>
    matchesResourceFilters(
      {
        name: event.deployable ?? "",
        namespace: event.namespace,
        project: event.project,
      },
      filters,
    ),
  );

  return (
    <Stack gap="lg">
      <PageHeader
        title="History"
        description="Pins, promotes, and rollbacks across every deployable."
      />
      {error != null && (
        <Alert color="red" title="API unavailable">
          {error}. Start the Go API with `just serve`.
        </Alert>
      )}
      {events.length > 0 ? (
        <ResourceFilterBar
          value={filters}
          onChange={setFilters}
          namespaces={uniqueSorted(events.map((event) => event.namespace))}
          projects={uniqueSorted(events.map((event) => event.project))}
        />
      ) : null}
      <ResourceTable
        headers={["When", "App", "Kind", "Stage", "Image", "Actor", "Commit"]}
        isEmpty={filtered.length === 0 && error == null}
        emptyMessage={
          events.length === 0
            ? "No pin or promote commits yet."
            : "No events match these filters."
        }
        minWidth={960}
      >
        {filtered.map((event) => (
          <Table.Tr key={`${event.commit}:${event.deployable}:${event.stage}`}>
            <Table.Td className="db-cell-fit">
              <RelativeTime value={event.at} />
            </Table.Td>
            <Table.Td>
              {event.deployable ? (
                <Text
                  component={Link}
                  to={`/deployables/${event.deployable}/history`}
                  fw={600}
                  c="var(--db-link)"
                >
                  {event.deployable}
                </Text>
              ) : (
                <Text size="sm" c="dimmed">
                  —
                </Text>
              )}
            </Table.Td>
            <Table.Td className="db-cell-fit">
              <EventKindBadge kind={event.kind} />
            </Table.Td>
            <Table.Td className="db-cell-fit">
              <Text size="sm">{event.stage}</Text>
            </Table.Td>
            <Table.Td className="db-cell-clip">
              <CompactImage value={event.image} empty="—" />
            </Table.Td>
            <Table.Td className="db-cell-clip">
              <HistoryActor actor={event.actor} author={event.author} />
            </Table.Td>
            <Table.Td className="db-cell-fit">
              <HistoryCommit hash={event.commit} url={event.commitURL} />
            </Table.Td>
          </Table.Tr>
        ))}
      </ResourceTable>
    </Stack>
  );
}

function HistoryCommit({ hash, url }: { hash?: string; url?: string }) {
  if (!hash) {
    return (
      <Text size="xs" c="dimmed">
        —
      </Text>
    );
  }
  const short = hash.slice(0, 7);
  if (url) {
    return (
      <Anchor
        href={url}
        size="xs"
        ff="monospace"
        target="_blank"
        rel="noreferrer"
        c="var(--db-link)"
      >
        {short}
      </Anchor>
    );
  }
  return (
    <Text size="xs" ff="monospace" c="dimmed">
      {short}
    </Text>
  );
}
