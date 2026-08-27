import { Alert, Group, Stack } from "@mantine/core";
import { useLocalStorage } from "@mantine/hooks";

import type { Route } from "./+types/home";
import { actorHeaders, listDeployables, submitPauseForm } from "~/lib/api.server";
import { CatalogView, CatalogViewToggle, type CatalogViewMode } from "~/ui/catalog-view";
import { PageHeader } from "~/ui/page-header";
import { PauseBanners, PauseButton } from "~/ui/pause-panel";
import {
  matchesResourceFilters,
  ResourceFilterBar,
  shouldRevalidateResourceFilters,
  uniqueSorted,
  useResourceFilters,
} from "~/ui/resource-filter";

export function meta(_args: Route.MetaArgs) {
  return [{ title: "Catalog · deploybot" }];
}

export const shouldRevalidate = shouldRevalidateResourceFilters;

export async function loader(_args: Route.LoaderArgs) {
  try {
    const data = await listDeployables();
    return {
      deployables: data.deployables,
      pause: data.pause,
      error: null as string | null,
    };
  } catch (err) {
    return {
      deployables: [],
      pause: undefined,
      error: err instanceof Error ? err.message : String(err),
    };
  }
}

export async function action({ request }: Route.ActionArgs) {
  const form = await request.formData();
  const pause = await submitPauseForm(form, actorHeaders(request));
  if (pause) return pause;
  return {
    ok: false as const,
    error: `unknown intent ${String(form.get("intent") ?? "")}`,
  };
}

export default function Home({ loaderData }: Route.ComponentProps) {
  const { deployables, pause, error } = loaderData;
  const [view, setView] = useLocalStorage<CatalogViewMode>({
    key: "deploybot-catalog-view",
    defaultValue: "cards",
    deserialize: (value) => (value === "table" ? "table" : "cards"),
  });
  const [filters, setFilters] = useResourceFilters();
  const filtered = deployables.filter((d) => matchesResourceFilters(d, filters));
  const stages = uniqueSorted(
    deployables.flatMap((d) => (d.stages ?? []).map((st) => st.name)),
  );

  return (
    <Stack gap="lg">
      <PageHeader
        title="Catalog"
        description="Apps deploybot can pin, promote, and reconcile."
        actions={
          <Group gap="sm">
            <PauseButton stages={stages} action="/" />
            <CatalogViewToggle value={view} onChange={setView} />
          </Group>
        }
      />
      {error != null && (
        <Alert color="red" title="API unavailable">
          {error}. Start the Go API with `just serve`.
        </Alert>
      )}
      <PauseBanners pause={pause} action="/" />
      {deployables.length > 0 ? (
        <ResourceFilterBar
          value={filters}
          onChange={setFilters}
          namespaces={uniqueSorted(deployables.map((d) => d.namespace))}
          projects={uniqueSorted(deployables.map((d) => d.project))}
        />
      ) : null}
      <CatalogView
        deployables={filtered}
        pause={pause}
        view={view}
        emptyMessage={
          deployables.length > 0
            ? "No deployables match these filters."
            : "No deployable specs found."
        }
      />
    </Stack>
  );
}
