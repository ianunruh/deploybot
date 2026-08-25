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
  const { status, stages, onRollback } = useOutletContext<DeployableContext>();
  const stageImages = Object.fromEntries(stages.map((st) => [st.name, st.image]));
  return (
    <ReleaseHistory
      stages={stages.map((st) => st.name)}
      releases={loaderData.history?.releases ?? []}
      imageRepo={status.imageRepo}
      stageImages={stageImages}
      error={loaderData.error}
      onRollback={onRollback}
    />
  );
}
