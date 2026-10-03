import { defineConfig, globalIgnores } from "eslint/config";
import nextCoreWebVitals from "eslint-config-next/core-web-vitals";
import nextTypeScript from "eslint-config-next/typescript";

export default defineConfig([
  {
    settings: {
      next: {
        rootDir: import.meta.dirname,
      },
    },
  },
  ...nextCoreWebVitals,
  ...nextTypeScript,
  globalIgnores([".next/**", "node_modules/**", "playwright-report/**", "test-results/**"]),
]);
