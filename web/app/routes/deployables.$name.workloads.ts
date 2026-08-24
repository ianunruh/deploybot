import type { Route } from "./+types/deployables.$name.workloads";
import { getDeployableWorkloads, type LiveWorkloads } from "~/lib/api.server";

export type WorkloadsLoaderData = LiveWorkloads & { error: string | null };

export async function loader({ params }: Route.LoaderArgs): Promise<WorkloadsLoaderData> {
  const name = params.name;
  if (!name) {
    throw new Response("Missing name", { status: 400 });
  }
  try {
    const data = await getDeployableWorkloads(name);
    return { ...data, error: null };
  } catch (err) {
    return {
      stages: [],
      error: err instanceof Error ? err.message : String(err),
    };
  }
}
