// Standalone on purpose: mergeConfig with vite.config.ts applies solid()
// twice and loads two copies of solid-js.
import { defineConfig } from "vitest/config";
import solid from "vite-plugin-solid";
import { fileURLToPath, URL } from "node:url";

export default defineConfig({
  plugins: [solid()],
  define: { __APP_VERSION__: JSON.stringify("test") },
  resolve: {
    alias: {
      "~": fileURLToPath(new URL("./src", import.meta.url)),
      "@catalystcal/client": fileURLToPath(new URL("../clients/typescript/index.ts", import.meta.url)),
    },
    conditions: ["development", "browser"],
  },
  test: {
    environment: "happy-dom",
    setupFiles: ["./vitest.setup.ts"],
    include: ["src/**/*.{test,spec}.{ts,tsx}"],
  },
});
