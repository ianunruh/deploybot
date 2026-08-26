import { useOutletContext } from "react-router";

import type { DeployableContext } from "./deployables.$name";
import { WorkloadPods } from "~/ui/workload-pods";

export default function DeployableWorkload() {
  const { stages } = useOutletContext<DeployableContext>();
  return <WorkloadPods stages={stages} />;
}
