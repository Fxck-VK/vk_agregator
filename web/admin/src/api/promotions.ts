import type { AdminApiPath, AdminClient } from "./adminClient";

export const promotionPlans = ["lite", "start", "pro", "ultima", "elite"] as const;
export type PromotionDraft = { code: string; percent: string; enabled: boolean; startsAt: string; endsAt: string; maxUses: string; planCodes: string[] };
export type PromotionConfig = { code: string; percent: number; enabled: boolean; starts_at: string; ends_at: string | null; max_uses: number; plan_codes: string[]; revision?: number };
export type Promotion = PromotionConfig & { id: string; revision: number; reserved_count: number; redeemed_count: number; created_at: string; updated_at: string };
export type PromotionPage = { items: Promotion[]; has_more: boolean };
export type PromotionAccess = { read: boolean; manage: boolean };
export type PromotionMutation = { path: AdminApiPath; key: string; body: PromotionConfig };

export function buildPromotionConfig(draft: PromotionDraft, existing?: Pick<Promotion, "code" | "revision">): PromotionConfig {
  const code = draft.code.trim();
  if (!/^[a-zA-Z0-9_-]{3,64}$/.test(code)) throw new Error("Код: от 3 до 64 латинских букв, цифр, _ или -.");
  const normalized = code.toUpperCase();
  if (existing && (normalized !== existing.code || !Number.isSafeInteger(existing.revision) || existing.revision < 1)) {
    throw new Error("Код неизменяем. Обновите список и выберите актуальную версию.");
  }
  const percent = integer(draft.percent, 1, 99, "Скидка: целое число от 1 до 99%.");
  const maxUses = integer(draft.maxUses, 0, Number.MAX_SAFE_INTEGER, "Лимит: неотрицательное целое число, 0 — без ограничений.");
  const startsAt = rfc3339(draft.startsAt);
  const endsAt = draft.endsAt.trim() ? rfc3339(draft.endsAt) : null;
  if (endsAt && Date.parse(endsAt) <= Date.parse(startsAt)) throw new Error("Окончание должно быть позже начала.");
  if (!draft.planCodes.length || draft.planCodes.some((plan) => !promotionPlans.some((allowed) => allowed === plan))) {
    throw new Error("Выберите хотя бы один допустимый тариф.");
  }
  return {
    code: normalized, percent, enabled: draft.enabled, starts_at: startsAt, ends_at: endsAt,
    max_uses: maxUses, plan_codes: promotionPlans.filter((plan) => draft.planCodes.includes(plan)),
    ...(existing ? { revision: existing.revision } : {}),
  };
}

function integer(raw: string, min: number, max: number, message: string): number {
  const value = Number(raw);
  if (!/^\d+$/.test(raw) || !Number.isSafeInteger(value) || value < min || value > max) throw new Error(message);
  return value;
}

function rfc3339(raw: string): string {
  const value = raw.trim();
  const match = /^(\d{4})-(\d{2})-(\d{2})T([01]\d|2[0-3]):[0-5]\d:[0-5]\d(?:\.\d{1,9})?(?:Z|[+-](?:[01]\d|2[0-3]):[0-5]\d)$/.exec(value);
  const millis = Date.parse(value);
  // Date.parse normalizes invalid dates such as February 31; reject them first.
  const calendar = match ? new Date(`${match[1]}-${match[2]}-${match[3]}T00:00:00Z`) : undefined;
  if (!match || !Number.isFinite(millis) || calendar?.toISOString().slice(0, 10) !== value.slice(0, 10)) {
    throw new Error("Укажите корректную дату RFC 3339 с часовым поясом, например 2026-10-31T00:00:00Z.");
  }
  return new Date(millis).toISOString();
}

export function promotionAccess(data: unknown): PromotionAccess {
  const denied = { read: false, manage: false };
  if (!data || typeof data !== "object" || !("effective_role" in data) || !("roles" in data)) return denied;
  if (typeof data.effective_role !== "string" || !["owner", "admin", "finance"].includes(data.effective_role) || !Array.isArray(data.roles)) return denied;
  const role = data.roles.find((candidate: unknown) => candidate && typeof candidate === "object" && "role" in candidate && candidate.role === data.effective_role);
  if (!role || !Array.isArray(role.permissions)) return denied;
  const read = role.permissions.includes("promotions:read");
  return { read, manage: read && role.permissions.includes("promotions:manage") };
}

export async function fetchPromotions(client: AdminClient, offset: number, signal?: AbortSignal): Promise<PromotionPage> {
  if (!Number.isSafeInteger(offset) || offset < 0) throw new Error("Некорректная страница.");
  const data = await client.request<PromotionPage>(`/admin/promotions?limit=50&offset=${offset}`, { signal });
  return { items: data.items ?? [], has_more: data.has_more };
}

const uuidPattern = /^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$/i;

export function preparePromotionMutation(body: PromotionConfig, id?: string): PromotionMutation {
  if (id && (!uuidPattern.test(id) || /^0{8}-0{4}-0{4}-0{4}-0{12}$/.test(id))) throw new Error("Некорректный идентификатор промокода.");
  const snapshot = { ...body, plan_codes: [...body.plan_codes] };
  Object.freeze(snapshot.plan_codes);
  Object.freeze(snapshot);
  // Promotions require a bare UUID; the legacy scoped admin keys are incompatible.
  const bytes = crypto.getRandomValues(new Uint8Array(16));
  bytes[6] = (bytes[6] & 0x0f) | 0x40;
  bytes[8] = (bytes[8] & 0x3f) | 0x80;
  const hex = Array.from(bytes, (byte) => byte.toString(16).padStart(2, "0")).join("");
  const key = `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
  return Object.freeze({ path: id ? `/admin/promotions/${id}` as const : "/admin/promotions", key, body: snapshot });
}
export function savePromotion(client: AdminClient, mutation: PromotionMutation): Promise<Promotion> { return client.request<Promotion>(mutation.path, { method: "POST", body: mutation.body, idempotencyKey: mutation.key }); }
