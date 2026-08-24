import type { Route } from "./+types/deployables.$name.workflows";
import { getDeployableWorkflows } from "~/lib/api.server";
import { WorkflowRuns } from "~/ui/workflow-runs";

export async function loader({ params }: Route.LoaderArgs) {
  const name = params.name;
  if (!name) throw new Response("Missing name", { status: 400 });
  try {
    return {
      workflows: await getDeployableWorkflows(name),
      error: null as string | null,
    };
  } catch (err) {
    return {
      workflows: null,
      error: err instanceof Error ? err.message : String(err),
    };
  }
}

export function shouldRevalidate({ formMethod }: { formMethod?: string }) {
  return formMethod === "POST";
}

export default function DeployableWorkflows({ loaderData }: Route.ComponentProps) {
  return <WorkflowRuns workflows={loaderData.workflows} error={loaderData.error} />;
}
