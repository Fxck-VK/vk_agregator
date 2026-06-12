import { describe, expect, it, vi } from "vitest";
import { AdminApiError, createAdminClient, createIdempotencyKey, toSafeAdminError } from "./adminClient";

const sensitiveToken = "admin-token-should-not-render";
const privateUrl = "https://storage.local/private/artifact?signature=secret";
const promptBody = "generate a private prompt body";

describe("admin api client", () => {
  it("sends admin token but never exposes it in safe errors", async () => {
    const fetchImpl = vi.fn<typeof fetch>(async (_input, init) => {
      const headers = init?.headers as Headers;
      expect(headers.get("X-Admin-Token")).toBe(sensitiveToken);
      return new Response(
        JSON.stringify({
          error: "provider failed",
          token: sensitiveToken,
          url: privateUrl,
          prompt: promptBody,
        }),
        { status: 502 },
      );
    });
    const client = createAdminClient({
      tokenProvider: () => sensitiveToken,
      fetchImpl,
      timeoutMs: 50,
    });

    await client.request("/admin/jobs").catch((error: unknown) => {
      const safe = toSafeAdminError(error);
      const rendered = JSON.stringify(safe);
      expect(safe.code).toBe("admin_server_error");
      expect(safe.message).toBe("Admin service is unavailable.");
      expect(safe.status).toBe(502);
      expect(rendered).not.toContain(sensitiveToken);
      expect(rendered).not.toContain(privateUrl);
      expect(rendered).not.toContain(promptBody);
    });
  });

  it("normalizes auth responses without raw payload text", async () => {
    const fetchImpl = vi.fn<typeof fetch>(async () => {
      return new Response(`raw ${sensitiveToken} ${privateUrl}`, { status: 401 });
    });
    const client = createAdminClient({
      tokenProvider: () => sensitiveToken,
      fetchImpl,
    });

    await client.request("/billing/payment-history").catch((error: unknown) => {
      expect(error).toBeInstanceOf(AdminApiError);
      const safe = toSafeAdminError(error);
      expect(safe.code).toBe("admin_auth_required");
      expect(safe.message).toBe("Admin authorization is required.");
      expect(safe.message).not.toContain(sensitiveToken);
      expect(safe.message).not.toContain(privateUrl);
    });
  });

  it("supports timeout as a safe timeout error", async () => {
    const client = createAdminClient({
      tokenProvider: () => sensitiveToken,
      timeoutMs: 1,
      fetchImpl: async (_input, init) =>
        new Promise<Response>((_resolve, reject) => {
          init?.signal?.addEventListener("abort", () => reject(new DOMException("aborted", "AbortError")));
        }),
    });

    await expect(client.request("/admin/jobs")).rejects.toMatchObject({
      code: "admin_timeout",
      message: "Admin request timed out.",
    });
  });

  it("adds idempotency keys without placing them in thrown messages", async () => {
    const idempotencyKey = createIdempotencyKey("payment refund");
    const fetchImpl = vi.fn<typeof fetch>(async (_input, init) => {
      const headers = init?.headers as Headers;
      expect(headers.get("X-Idempotency-Key")).toBe(idempotencyKey);
      return new Response(JSON.stringify({ error: idempotencyKey }), { status: 409 });
    });
    const client = createAdminClient({
      tokenProvider: () => sensitiveToken,
      fetchImpl,
    });

    await client
      .request("/billing/payment-intents/00000000-0000-0000-0000-000000000000/refund", {
        method: "POST",
        idempotencyKey,
        body: { reason: "operator requested" },
      })
      .catch((error: unknown) => {
        const safe = toSafeAdminError(error);
        expect(safe.code).toBe("admin_bad_request");
        expect(safe.message).not.toContain(idempotencyKey);
      });
  });
});
