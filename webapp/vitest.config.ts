// Standalone: vite.config.ts reads version/VERSION.txt and sets the dev
// proxy, which tests do not need.
import { defineConfig } from "vitest/config";
import { fileURLToPath, URL } from "node:url";

export default defineConfig({
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
    include: ["src/**/*.{test,spec}.ts"],
  },
});
