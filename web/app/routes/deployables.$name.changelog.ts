import type { Route } from "./+types/deployables.$name.changelog";
import { getDeployableChangelog, type Changelog } from "~/lib/api.server";

export type ChangelogLoaderData = {
  changelog: Changelog | null;
  error: string | null;
};

export async function loader({
  params,
  request,
}: Route.LoaderArgs): Promise<ChangelogLoaderData> {
  const name = params.name;
  if (!name) {
    throw new Response("Missing name", { status: 400 });
  }
  const url = new URL(request.url);
  const from = url.searchParams.get("from") ?? "";
  const to = url.searchParams.get("to") ?? "";
  try {
    return { changelog: await getDeployableChangelog(name, from, to), error: null };
  } catch (err) {
    return {
      changelog: null,
      error: err instanceof Error ? err.message : String(err),
    };
  }
}
