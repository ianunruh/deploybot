import { type RouteConfig, index, route } from "@react-router/dev/routes";

export default [
  index("routes/home.tsx"),
  route("deployables/:name", "routes/deployables.$name.tsx"),
  route("deployables/:name/sync/:stage", "routes/deployables.$name.sync.$stage.tsx"),
] satisfies RouteConfig;
