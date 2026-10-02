import { expect, it } from "vitest";
import { ReadError, classifyReadError, readJson } from "./read-error";

it.each([[401, "session", false], [403, "forbidden", false], [429, "rate_limit", true], [503, "unavailable", true], [400, "invalid_request", false]])("classifies HTTP %i without retaining response bodies", async (status, kind, retryable) => {
  const error = await readJson(new Response("private details", { status: Number(status), headers: { "Retry-After": "3" } }), value => value).catch(error => error);
  expect(error).toBeInstanceOf(ReadError);
  expect(error).toMatchObject({ kind, retryable });
  expect((error as Error).message).not.toContain("private details");
});
it("distinguishes timeout, network and invalid payload", async () => {
  expect(classifyReadError(new DOMException("", "TimeoutError")).kind).toBe("timeout");
  expect(classifyReadError(new TypeError("Failed to fetch")).kind).toBe("network");
  await expect(readJson(Response.json({}), () => { throw new Error("private payload"); })).rejects.toMatchObject({ kind: "invalid_payload", retryable: false });
});
