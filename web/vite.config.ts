import { reactRouter } from "@react-router/dev/vite";
import { defineConfig } from "vite";

export default defineConfig({
  plugins: [reactRouter()],
  resolve: {
    tsconfigPaths: true,
  },
  ssr: {
    noExternal: [
      "@mantine/core",
      "@mantine/hooks",
      "@mantine/notifications",
      "@fontsource/geist-mono",
    ],
  },
  server: {
    port: 5173,
  },
});
