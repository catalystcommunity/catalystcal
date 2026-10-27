import { defineConfig } from "vite";
import solid from "vite-plugin-solid";
import { fileURLToPath, URL } from "node:url";
import { readFileSync } from "node:fs";

// version/VERSION.txt at the repo root owns the release version.
const appVersion = readFileSync(
  fileURLToPath(new URL("../version/VERSION.txt", import.meta.url)),
  "utf8",
).trim();

// The Go API listens on 7080 in dev (CATALYSTCAL_API_PORT).
const api = "http://localhost:7080";

export default defineConfig({
  define: {
    __APP_VERSION__: JSON.stringify(appVersion),
  },
  plugins: [solid()],
  resolve: {
    alias: {
      "~": fileURLToPath(new URL("./src", import.meta.url)),
      // The generated client lives at repo-root clients/typescript. Alias to
      // its source barrel so Vite transpiles it like in-tree TS.
      "@catalystcal/client": fileURLToPath(new URL("../clients/typescript/index.ts", import.meta.url)),
    },
  },
  server: {
    // clients/ is outside the Vite root; let the dev server read it.
    fs: { allow: [fileURLToPath(new URL("..", import.meta.url))] },
    port: 5180,
    strictPort: true,
    host: "localhost",
    proxy: {
      "/api": { target: api, changeOrigin: false },
      "/ics": { target: api, changeOrigin: false },
    },
  },
  build: {
    target: "es2022",
    sourcemap: true,
  },
});
