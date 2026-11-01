#!/usr/bin/env node
import { open } from "node:fs/promises";
import { pathToFileURL } from "node:url";

const plans = ["lite", "start", "pro", "ultima", "elite"];
const uuid = /^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$/i;
const zeroUUID = "00000000-0000-0000-0000-000000000000";
class PromotionCLIError extends Error {}
function fail(message) { throw new PromotionCLIError(message); }

const help = `Protected promotion HTTP tooling (Node.js 22+)
  node scripts/dev/promotions.mjs list [--offset 0]
  node scripts/dev/promotions.mjs create --file <config.json> [--confirm]
  node scripts/dev/promotions.mjs update --id <UUID> --file <config.json> [--confirm]
Create/update without --confirm only validate and print a review; no HTTP calls.
HTTP environment: EPIFERA_PROMOTIONS_BASE_URL, EPIFERA_PROMOTIONS_TOKEN.
Confirmed writes also require EPIFERA_PROMOTIONS_REQUEST_KEY (one private UUID).
Retry with the same key, file and command. Never put credentials in arguments.
No direct database access. No automatic retry, campaign or key generation.`;

function command(argv) {
  if (argv.length === 1 && argv[0] === "--help") return { operation: "help" };
  const [operation, ...args] = argv;
  if (!["list", "create", "update"].includes(operation)) fail("Choose list, create, update or --help.");
  const options = {};
  for (let index = 0; index < args.length; index++) {
    const name = args[index];
    if (!["--file", "--id", "--offset", "--confirm"].includes(name) || name in options) fail("Invalid or duplicate argument. Credentials are environment-only.");
    if (name === "--confirm") options[name] = true;
    else {
      if (!args[index + 1] || args[index + 1].startsWith("--")) fail("Missing option value.");
      options[name] = args[++index];
    }
  }
  if (operation === "list") {
    if (Object.keys(options).some((name) => name !== "--offset")) fail("List accepts only --offset.");
    const offset = options["--offset"] ?? "0";
    if (!/^\d+$/.test(offset) || !Number.isSafeInteger(Number(offset))) fail("Invalid list offset.");
    return { operation, offset: Number(offset) };
  }
  if (!options["--file"] || "--offset" in options) fail("A bounded JSON configuration file is required.");
  if (operation === "update" && (!uuid.test(options["--id"] ?? "") || options["--id"] === zeroUUID)) fail("Update requires a nonzero UUID.");
  if (operation === "create" && "--id" in options) fail("Create must not include an id.");
  return { operation, file: options["--file"], id: options["--id"], confirmed: Boolean(options["--confirm"]) };
}

function date(value) {
  if (typeof value !== "string" || !/^\d{4}-\d{2}-\d{2}T(?:[01]\d|2[0-3]):[0-5]\d:[0-5]\d(?:\.\d{1,9})?(?:Z|[+-](?:[01]\d|2[0-3]):[0-5]\d)$/.test(value)) fail("Dates require RFC 3339 with a timezone.");
  const millis = Date.parse(value);
  const calendar = new Date(`${value.slice(0, 10)}T00:00:00Z`);
  if (!Number.isFinite(millis) || !Number.isFinite(calendar.getTime()) || calendar.toISOString().slice(0, 10) !== value.slice(0, 10)) fail("Invalid calendar date.");
  return new Date(millis).toISOString();
}

function config(body, update) {
  const allowed = ["code", "percent", "enabled", "starts_at", "ends_at", "max_uses", "plan_codes", ...(update ? ["revision"] : [])];
  if (!body || typeof body !== "object" || Array.isArray(body) || Object.keys(body).some((key) => !allowed.includes(key))) fail("Invalid promotion fields.");
  if (typeof body.code !== "string" || !/^[a-zA-Z0-9_-]{3,64}$/.test(body.code.trim())) fail("Code requires 3..64 ASCII letters, digits, underscores or hyphens.");
  if (!Number.isInteger(body.percent) || body.percent < 1 || body.percent > 99) fail("Percent requires an integer in 1..99.");
  if (typeof body.enabled !== "boolean") fail("Enabled requires an explicit boolean.");
  if (!Number.isSafeInteger(body.max_uses) || body.max_uses < 0) fail("Max uses requires a nonnegative safe integer; zero is unlimited.");
  if (!Array.isArray(body.plan_codes) || !body.plan_codes.length || body.plan_codes.some((plan) => !plans.includes(plan))) fail("Select at least one supported tariff.");
  if (update && (!Number.isSafeInteger(body.revision) || body.revision < 1)) fail("Update requires the current positive revision and unchanged code.");
  const startsAt = date(body.starts_at);
  const endsAt = body.ends_at == null ? null : date(body.ends_at);
  if (endsAt && Date.parse(endsAt) <= Date.parse(startsAt)) fail("End must follow start.");
  return {
    code: body.code.trim().toUpperCase(), percent: body.percent, enabled: body.enabled,
    starts_at: startsAt, ends_at: endsAt, max_uses: body.max_uses,
    plan_codes: plans.filter((plan) => body.plan_codes.includes(plan)),
    ...(update ? { revision: body.revision } : {}),
  };
}

async function readConfigFile(file) {
  let handle;
  try {
    handle = await open(file, "r");
    const buffer = Buffer.alloc(8193);
    const { bytesRead } = await handle.read(buffer, 0, buffer.length, 0);
    if (bytesRead > 8192) fail("Promotion configuration exceeds 8192 bytes.");
    return buffer.subarray(0, bytesRead).toString("utf8");
  } catch (error) {
    if (error instanceof PromotionCLIError) throw error;
    fail("Cannot read promotion configuration file.");
  } finally { await handle?.close(); }
}

function connection(env, writing) {
  const token = env.EPIFERA_PROMOTIONS_TOKEN?.trim();
  if (!token || /[\r\n]/.test(token)) fail("A private EPIFERA_PROMOTIONS_TOKEN environment value is required.");
  let url;
  try { url = new URL(env.EPIFERA_PROMOTIONS_BASE_URL); } catch { fail("A valid EPIFERA_PROMOTIONS_BASE_URL origin is required."); }
  const loopback = ["localhost", "127.0.0.1", "[::1]"].includes(url.hostname);
  if ((url.protocol !== "https:" && !(url.protocol === "http:" && loopback)) || url.username || url.password || url.search || url.hash || url.pathname !== "/") fail("Use an HTTPS origin (or loopback HTTP), with no credentials, path, query or fragment.");
  const key = env.EPIFERA_PROMOTIONS_REQUEST_KEY?.trim();
  if (writing && (!uuid.test(key ?? "") || key === zeroUUID)) fail("Confirmed writes require one stable UUID in EPIFERA_PROMOTIONS_REQUEST_KEY.");
  return { origin: url.origin, token, key };
}

function canAccess(data, writing) {
  if (!["owner", "admin", "finance"].includes(data?.effective_role) || !Array.isArray(data?.roles)) return false;
  const permissions = data.roles.find((role) => role?.role === data.effective_role)?.permissions;
  return Array.isArray(permissions) && permissions.includes("promotions:read") && (!writing || permissions.includes("promotions:manage"));
}

function publicPromotion(item) {
  if (!item || !uuid.test(item.id ?? "") || item.id === zeroUUID) fail("Invalid promotion response.");
  const fields = Object.fromEntries(["code", "percent", "enabled", "starts_at", "ends_at", "max_uses", "plan_codes", "revision"].map((key) => [key, item[key]]));
  const safe = config(fields, true);
  if (![item.reserved_count, item.redeemed_count].every((value) => Number.isSafeInteger(value) && value >= 0)) fail("Invalid promotion counters.");
  return { id: item.id, ...safe, reserved_count: item.reserved_count, redeemed_count: item.redeemed_count, created_at: date(item.created_at), updated_at: date(item.updated_at) };
}

export async function runPromotions(argv, { env = process.env, fetchImpl = fetch, readConfig = readConfigFile } = {}) {
  try {
    const parsed = command(argv);
    if (parsed.operation === "help") return { help };
    let body;
    if (parsed.operation !== "list") {
      const raw = await readConfig(parsed.file);
      if (typeof raw !== "string" || Buffer.byteLength(raw) > 8192) fail("Promotion configuration exceeds 8192 bytes.");
      let value;
      try { value = JSON.parse(raw); } catch { fail("Promotion configuration must be valid JSON."); }
      body = config(value, parsed.operation === "update");
      if (!parsed.confirmed) return { mode: "review_only", operation: parsed.operation, ...(parsed.id ? { id: parsed.id } : {}), config: body };
    }
    const writing = parsed.operation !== "list";
    const { origin, token, key } = connection(env, writing);
    async function request(path, payload) {
      const result = await fetchImpl(`${origin}${path}`, {
        method: payload ? "POST" : "GET", redirect: "error", credentials: "omit", cache: "no-store",
        signal: AbortSignal.timeout(10_000),
        headers: { Accept: "application/json", "X-Admin-Token": token, ...(payload ? { "Content-Type": "application/json", "X-Idempotency-Key": key } : {}) },
        ...(payload ? { body: JSON.stringify(payload) } : {}),
      });
      if (!result.ok) {
        if (result.status === 409) fail("Promotion conflict: refresh the list and check code/revision; do not blindly change the request key.");
        if (result.status === 401 || result.status === 403) fail("Promotion permission denied.");
        if (result.status === 400 || result.status === 404) fail("Promotion request rejected; review the configuration and target.");
        fail("Promotion outcome is unconfirmed. Retry the same command, file and request key.");
      }
      return await result.json();
    }
    if (!canAccess(await request("/admin/access/operator"), writing)) fail("Promotion permission denied by the effective access contract.");
    if (writing) return { mode: "saved", item: publicPromotion(await request(parsed.id ? `/admin/promotions/${parsed.id}` : "/admin/promotions", body)) };
    const page = await request(`/admin/promotions?limit=50&offset=${parsed.offset}`);
    const items = page?.items ?? [];
    if (!Array.isArray(items) || items.length > 50 || typeof page?.has_more !== "boolean") fail("Invalid bounded promotion list.");
    return { items: items.map(publicPromotion), has_more: page.has_more, offset: parsed.offset };
  } catch (error) {
    if (error instanceof PromotionCLIError) throw error;
    fail("Promotion outcome is unconfirmed. Retry unchanged; no transport or response details are printed.");
  }
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  runPromotions(process.argv.slice(2)).then((result) => {
    process.stdout.write(result.help ? `${result.help}\n` : `${JSON.stringify(result, null, 2)}\n`);
  }).catch((error) => { process.stderr.write(`${error.message}\n`); process.exitCode = 1; });
}
