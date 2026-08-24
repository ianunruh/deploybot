import { useOutletContext } from "react-router";

import type { Route } from "./+types/deployables.$name.history";
import type { DeployableContext } from "./deployables.$name";
import { getDeployableHistory } from "~/lib/api.server";
import { ReleaseHistory } from "~/ui/release-history";

export async function loader({ params }: Route.LoaderArgs) {
  const name = params.name;
  if (!name) throw new Response("Missing name", { status: 400 });
  try {
    return {
      history: await getDeployableHistory(name),
      error: null as string | null,
    };
  } catch (err) {
    return {
      history: null,
      error: err instanceof Error ? err.message : String(err),
    };
  }
}

export function shouldRevalidate({ formMethod }: { formMethod?: string }) {
  return formMethod === "POST";
}

export default function DeployableHistory({ loaderData }: Route.ComponentProps) {
  const { stages } = useOutletContext<DeployableContext>();
  return (
    <ReleaseHistory
      stages={stages.map((st) => st.name)}
      releases={loaderData.history?.releases ?? []}
      error={loaderData.error}
    />
  );
}
