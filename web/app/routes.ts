import { type RouteConfig, index, route } from "@react-router/dev/routes";

export default [
  index("routes/home.tsx"),
  route("healthz", "routes/healthz.ts"),
  route("history", "routes/history.tsx"),
  route("updates", "routes/updates.tsx"),
  route("deployables/:name", "routes/deployables.$name.tsx", [
    index("routes/deployables.$name._index.tsx"),
    route("history", "routes/deployables.$name.history.tsx"),
    route("workload", "routes/deployables.$name.workload.tsx"),
    route("workflows", "routes/deployables.$name.workflows.tsx"),
  ]),
  route("deployables/:name/images", "routes/deployables.$name.images.ts"),
  route("deployables/:name/workloads", "routes/deployables.$name.workloads.ts"),
  route("deployables/:name/changelog", "routes/deployables.$name.changelog.ts"),
  route(
    "deployables/:name/reconcile/:stage",
    "routes/deployables.$name.reconcile.$stage.tsx",
  ),
] satisfies RouteConfig;
