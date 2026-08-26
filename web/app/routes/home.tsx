import { Alert, Stack } from "@mantine/core";
import { useLocalStorage } from "@mantine/hooks";

import type { Route } from "./+types/home";
import { listDeployables } from "~/lib/api.server";
import { CatalogView, CatalogViewToggle, type CatalogViewMode } from "~/ui/catalog-view";
import { PageHeader } from "~/ui/page-header";

export function meta(_args: Route.MetaArgs) {
  return [{ title: "Catalog · deploybot" }];
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
  const [view, setView] = useLocalStorage<CatalogViewMode>({
    key: "deploybot-catalog-view",
    defaultValue: "cards",
    deserialize: (value) => (value === "table" ? "table" : "cards"),
  });

  return (
    <Stack gap="lg">
      <PageHeader
        title="Catalog"
        description="Apps deploybot can pin, promote, and reconcile."
        actions={<CatalogViewToggle value={view} onChange={setView} />}
      />
      {error != null && (
        <Alert color="red" title="API unavailable">
          {error}. Start the Go API with `just serve`.
        </Alert>
      )}
      <CatalogView deployables={deployables} view={view} />
    </Stack>
  );
}
