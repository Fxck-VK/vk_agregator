import { test } from "node:test";
import assert from "node:assert/strict";
import { runPromotions } from "./promotions.mjs";

const id = "10000000-0000-4000-8000-000000000001";
const key = "20000000-0000-4000-8000-000000000002";
const token = "private-test-token";
const config = { code: " sample_25 ", percent: 25, enabled: false, starts_at: "2026-10-31T00:00:00Z", ends_at: null, max_uses: 100, plan_codes: ["pro"] };
const env = { EPIFERA_PROMOTIONS_BASE_URL: "https://ops.example.test", EPIFERA_PROMOTIONS_TOKEN: token, EPIFERA_PROMOTIONS_REQUEST_KEY: key };
const access = { effective_role: "finance", roles: [{ role: "finance", permissions: ["promotions:read", "promotions:manage"] }] };
const dto = { ...config, code: "SAMPLE_25", id, revision: 1, reserved_count: 0, redeemed_count: 0, created_at: "2026-10-31T00:00:00Z", updated_at: "2026-10-31T00:00:00Z" };
const readConfig = async () => JSON.stringify(config);
const response = (body, status = 200) => new Response(JSON.stringify(body), { status });

test("create defaults to review only, needs no credential and makes no HTTP request", async () => {
  let calls = 0;
  const result = await runPromotions(["create", "--file", "request.json"], { env: {}, readConfig, fetchImpl: async () => { calls++; throw new Error("network forbidden"); } });
  assert.equal(calls, 0);
  assert.equal(result.mode, "review_only");
  assert.equal(result.config.code, "SAMPLE_25");
  assert.equal(result.config.enabled, false);
});

test("confirmed retries share an explicit environment UUID and unchanged body", async () => {
  const requests = [];
  const fetchImpl = async (url, options) => {
    requests.push({ url, ...options });
    return response(url.endsWith("/access/operator") ? access : dto);
  };
  for (let attempt = 0; attempt < 2; attempt++) await runPromotions(["create", "--file", "request.json", "--confirm"], { env, readConfig, fetchImpl });
  const writes = requests.filter((item) => item.method === "POST");
  assert.equal(writes.length, 2);
  assert.equal(writes[0].headers["X-Admin-Token"], token);
  assert.equal(writes[0].headers["X-Idempotency-Key"], key);
  assert.equal(writes[0].body, writes[1].body);
  assert.equal(writes[0].redirect, "error");
  assert.equal(writes[0].url, "https://ops.example.test/admin/promotions");
});

test("update carries revision to the UUID path", async () => {
  const requests = [];
  await runPromotions(["update", "--id", id, "--file", "request.json", "--confirm"], {
    env, readConfig: async () => JSON.stringify({ ...config, revision: 4 }),
    fetchImpl: async (url, options) => { requests.push({ url, ...options }); return response(url.endsWith("/access/operator") ? access : dto); },
  });
  assert.equal(requests[1].url, `https://ops.example.test/admin/promotions/${id}`);
  assert.equal(JSON.parse(requests[1].body).revision, 4);
});

test("list is bounded, checks effective access and projects safe DTO fields", async () => {
  const urls = [];
  const result = await runPromotions(["list", "--offset", "50"], {
    env, fetchImpl: async (url) => { urls.push(url); return response(url.endsWith("/access/operator") ? access : { items: [{ ...dto, token }], has_more: false }); },
  });
  assert.equal(urls[1], "https://ops.example.test/admin/promotions?limit=50&offset=50");
  assert.equal(result.items[0].code, "SAMPLE_25");
  assert.ok(!JSON.stringify(result).includes(token));
});

test("forbidden effective roles never reach a write", async () => {
  let calls = 0;
  await assert.rejects(runPromotions(["create", "--file", "request.json", "--confirm"], {
    env, readConfig, fetchImpl: async () => { calls++; return response({ ...access, effective_role: "support" }); },
  }), /permission/i);
  assert.equal(calls, 1);
});

test("rejects unsafe destinations, credentials in arguments and missing stable keys before any HTTP", async () => {
  const badEnvs = [
    { ...env, EPIFERA_PROMOTIONS_BASE_URL: "http://public.example.test" },
    { ...env, EPIFERA_PROMOTIONS_BASE_URL: "https://user:secret@ops.example.test" },
    { ...env, EPIFERA_PROMOTIONS_BASE_URL: "https://ops.example.test?token=secret" },
    { ...env, EPIFERA_PROMOTIONS_BASE_URL: "https://ops.example.test/admin" },
    { ...env, EPIFERA_PROMOTIONS_REQUEST_KEY: "" },
    { ...env, EPIFERA_PROMOTIONS_TOKEN: "" },
  ];
  let calls = 0;
  const fetchImpl = async () => { calls++; return response(dto); };
  for (const badEnv of badEnvs) await assert.rejects(runPromotions(["create", "--file", "request.json", "--confirm"], { env: badEnv, readConfig, fetchImpl }));
  await assert.rejects(runPromotions(["list", "--token", token], { env, fetchImpl }));
  assert.equal(calls, 0);
});

test("rejects invalid config, timestamps, unknown fields and omitted update revisions", async () => {
  for (const changes of [{ code: "aßb" }, { percent: 0 }, { percent: 100 }, { percent: 1.5 }, { enabled: "true" }, { max_uses: -1 }, { plan_codes: [] }, { plan_codes: ["max"] }, { starts_at: "2026-02-31T00:00:00Z" }, { starts_at: "2026-10-31T00:00:00" }, { ends_at: "2026-10-31T00:00:00Z" }, { token }]) {
    await assert.rejects(runPromotions(["create", "--file", "request.json"], { env: {}, readConfig: async () => JSON.stringify({ ...config, ...changes }) }));
  }
  await assert.rejects(runPromotions(["update", "--id", id, "--file", "request.json"], { env: {}, readConfig }));
  await assert.rejects(runPromotions(["create", "--file", "request.json"], { env: {}, readConfig: async () => " ".repeat(8193) }));
});

test("HTTP and transport failures never disclose raw response data, destination, token or request key", async () => {
  for (const status of [400, 401, 403, 409, 429, 503]) {
    await assert.rejects(runPromotions(["list"], { env, fetchImpl: async () => response({ token, key, raw: "private" }, status) }), (error) => {
      assert.ok(!String(error).includes(token));
      assert.ok(!String(error).includes(key));
      assert.ok(!String(error).includes("private"));
      return true;
    });
  }
  await assert.rejects(runPromotions(["list"], { env, fetchImpl: async () => { throw new Error(`${token} ${key}`); } }), (error) => !String(error).includes(token) && !String(error).includes(key));
});
