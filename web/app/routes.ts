import { type RouteConfig, index, route } from "@react-router/dev/routes";

export default [
  index("routes/home.tsx"),
  route("deployables/:name", "routes/deployables.$name.tsx"),
  route("deployables/:name/images", "routes/deployables.$name.images.ts"),
  route(
    "deployables/:name/reconcile/:stage",
    "routes/deployables.$name.reconcile.$stage.tsx",
  ),
] satisfies RouteConfig;
