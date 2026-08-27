import type { Route } from "./+types/ops.$cluster.$id.logs";
import { actorHeaders, apiURL, opsLogsPath } from "~/lib/api.server";

export async function loader({ params, request }: Route.LoaderArgs) {
  const cluster = params.cluster ?? "";
  const id = params.id ?? "";
  const res = await fetch(apiURL(opsLogsPath(cluster, id, true)), {
    headers: {
      Accept: "text/plain",
      ...actorHeaders(request),
    },
  });
  return new Response(res.body, {
    status: res.status,
    headers: {
      "Content-Type": res.headers.get("Content-Type") ?? "text/plain; charset=utf-8",
      "Cache-Control": "no-cache",
      "X-Accel-Buffering": "no",
    },
  });
}
