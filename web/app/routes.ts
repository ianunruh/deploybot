import { type RouteConfig, index, route } from "@react-router/dev/routes";

export default [
  index("routes/home.tsx"),
  route("deployables/:name", "routes/deployables.$name.tsx"),
] satisfies RouteConfig;
