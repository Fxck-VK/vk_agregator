import { existsSync } from "node:fs";
import { resolve } from "node:path";
import { expect, it } from "vitest";

it("does not install a route-level loading UI for workspace navigation", () => {
  expect(existsSync(resolve(process.cwd(), "src/app/app/loading.tsx"))).toBe(false);
  expect(existsSync(resolve(process.cwd(), "src/app/app/loading.module.css"))).toBe(false);
});
