import { defineConfig } from "vitest/config";
import path from "path";
import { fileURLToPath } from "url";

import { loadEnvConfig } from "@next/env";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

loadEnvConfig(__dirname);

export default defineConfig({
  test: {
    environment: "node",
    alias: {
      "@": path.resolve(__dirname, "./"),
    },
  },
});
