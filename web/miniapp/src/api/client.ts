// src/api/client.ts
// Тонкий типизированный клиент к /miniapp/* эндпоинтам.
// Все пути ОТНОСИТЕЛЬНЫЕ — уходят через Vite-proxy на :8080.

import bridge from "@vkontakte/vk-bridge";

/** Mirrors internal/adapter/inbound/miniapp JobDTO */
export interface Job {
  id: string;
  operation: string;
  modality: string;
  status: string;
  prompt?: string;
  conversation_id?: string;
  model_id?: string;
  model_name?: string;
  video_route_alias?: string;
  image_quality?: string;
  /** Backend-owned job estimate for display/history only. */
  cost_estimate: number;
  /** Backend-owned captured credits for display/history only. */
  cost_captured: number;
  output_artifact_ids: string[];
  error_code?: string;
  created_at: string;
  updated_at: string;
}

/** Public CreateJobRequest: no price/cost/provider/provider_cost/multiplier fields. */
export interface CreateJobInput {
  operation: string;
  prompt: string;
  model_id?: string;
  video_route_alias?: string;
  image_quality?: string;
  reference_artifact_ids?: string[];
  /** video_generate only: backend route-specific allowed durations */
  duration_sec?: number;
}

export interface CreateChatMessageInput {
  prompt: string;
  conversation_id?: string;
}

export interface ChatConversation {
  id: string;
  title: string;
  last_message_preview?: string;
  last_message_role?: "user" | "bot";
  created_at: string;
  updated_at: string;
}

export interface ChatConversationMessage {
  id: string;
  job_id: string;
  seq: number;
  role: "user" | "bot";
  text: string;
  created_at: string;
}

export interface CreateJobOptions {
  idempotencyKey: string;
}

/** Public estimate request: no price/cost/provider/provider_cost/multiplier fields. */
export interface EstimateInput {
  operation: string;
  prompt: string;
  model_id?: string;
  video_route_alias?: string;
  image_quality?: string;
  reference_artifact_ids?: string[];
  duration_sec?: number;
}

export interface EstimateResponse {
  operation: string;
  model_id?: string;
  model_name?: string;
  video_route_alias?: string;
  image_quality?: string;
  /** Exact backend-owned cost for this normalized request. */
  cost_estimate: number;
  balance_credits: number;
  enough_credits: boolean;
}

/** Public catalog item: estimate_credits is a backend display hint only. */
export interface ModelCatalogItem {
  type: "image" | "video";
  id: string;
  alias?: string;
  name: string;
  description?: string;
  /** Backend-provided display hint; not a frontend pricing source. */
  estimate_credits?: number;
  enabled: boolean;
  quality_options?: string[];
  default_quality?: string;
  allowed_durations_sec?: number[];
  allowed_resolutions?: string[];
  allowed_aspect_ratios?: string[];
  default_duration_sec?: number;
  default_resolution?: string;
  default_aspect_ratio?: string;
  requires_start_image: boolean;
  supports_reference_image: boolean;
  max_reference_images?: number;
}

/** Mirrors internal/adapter/inbound/miniapp BalanceDTO */
export interface BalanceResponse {
  balance_credits: number;
}

export interface ReferralInfo {
  code: string;
  invite_url: string;
  invited_count: number;
  registered_count: number;
  activated_count: number;
  rewarded_count: number;
  referrer_signup_reward_credits: number;
  referred_signup_reward_credits: number;
}

export interface ApplyReferralResponse {
  applied: boolean;
  already_applied: boolean;
  invalid_code: boolean;
  self_referral: boolean;
}

export interface PaymentProduct {
  id: string;
  code: string;
  title: string;
  amount: number;
  currency: string;
  credits: number;
  price_version: number;
}

export interface PaymentIntent {
  id: string;
  product_id?: string;
  status: string;
  amount: number;
  currency: string;
  credits: number;
  price_version: number;
  confirmation_url?: string;
  reused_active_payment?: boolean;
  notice?: string;
  created_at: string;
  updated_at: string;
}

export interface CreatePaymentIntentInput {
  product_code: string;
  receipt_email?: string;
  receipt_phone?: string;
  return_url?: string;
  force_new?: boolean;
}

export interface PaymentProductListResponse {
  items: PaymentProduct[];
  pagination: {
    limit: number;
    offset: number;
    count: number;
    has_more: boolean;
  };
}

export interface PaymentIntentListResponse {
  items: PaymentIntent[];
  pagination: {
    limit: number;
    offset: number;
    count: number;
    has_more: boolean;
  };
}

export interface ArtifactUploadResponse {
  artifact_id: string;
}

export interface JobListResponse {
  items: Job[];
  pagination: {
    limit: number;
    offset: number;
    count: number;
    has_more: boolean;
  };
}

export interface ChatConversationListResponse {
  items: ChatConversation[];
  pagination: {
    limit: number;
    offset: number;
    count: number;
    has_more: boolean;
  };
}

export interface ChatConversationMessageListResponse {
  items: ChatConversationMessage[];
  pagination: {
    limit: number;
    offset: number;
    count: number;
    has_more: boolean;
  };
}

export type ApiErrorCode =
  | "validation_error"
  | "unsupported_model"
  | "reference_artifacts_required"
  | "reference_artifacts_unsupported"
  | "too_many_reference_artifacts"
  | "media_upload_invalid"
  | "media_upload_too_large"
  | "media_upload_unsupported"
  | "media_provider_output_invalid"
  | "media_processing_unavailable"
  | "media_delivery_failed"
  | "media_overloaded_retry_later"
  | "auth_error"
  | "insufficient_credits"
  | "payment_cancel_unavailable"
  | "rate_limited"
  | "artifact_unavailable"
  | "service_unavailable"
  | "network_error"
  | "unknown_error";

export class ApiError extends Error {
  code: ApiErrorCode;
  backendError?: string;
  retryAfter?: string;
  userMessage: string;

  constructor(
    public status: number,
    code: ApiErrorCode,
    options: {
      backendError?: string;
      retryAfter?: string;
    } = {},
  ) {
    const userMessage = apiErrorMessageForCode(code);
    super(userMessage);
    this.name = "ApiError";
    this.code = code;
    this.backendError = options.backendError;
    this.retryAfter = options.retryAfter;
    this.userMessage = userMessage;
  }
}

type ClientEventType = "api_failure" | "api_latency" | "js_error" | "launch_failure" | "payment_flow_error" | "ui_event";

interface ClientTelemetryEvent {
  event_type: ClientEventType;
  surface?: "vk_mini_app";
  screen?: string;
  route?: string;
  status?: string;
  error_class?: string;
  step?: string;
  reason?: string;
  duration_ms?: number;
}

const TELEMETRY_ENABLED = import.meta.env.VITE_FRONTEND_TELEMETRY_ENABLED === "true";
export const MINIAPP_PAYMENT_CANCEL_ENABLED = import.meta.env.VITE_FEATURE_MINIAPP_PAYMENT_CANCEL_ENABLED !== "false";
export const MINIAPP_TOPUP_CATALOG_DROPDOWN_ENABLED =
  import.meta.env.VITE_FEATURE_MINIAPP_TOPUP_CATALOG_DROPDOWN_ENABLED !== "false";
export const MINIAPP_DARK_THEME_ONLY_ENABLED = import.meta.env.VITE_FEATURE_MINIAPP_DARK_THEME_ONLY_ENABLED !== "false";
export const MINIAPP_TOPUP_HISTORY_DROPDOWN_ENABLED =
  import.meta.env.VITE_FEATURE_MINIAPP_TOPUP_HISTORY_DROPDOWN_ENABLED !== "false";

let telemetryInstalled = false;
const appStartedAt = performance.now();

export function telemetryRoute(path: string): string {
  const [withoutQuery] = path.split(/[?#]/, 1);
  return withoutQuery
    .split("/")
    .map((part) => (ARTIFACT_ID_RE.test(part) ? ":id" : /^\d+$/.test(part) ? ":id" : part))
    .join("/");
}

export function telemetryLabel(value: string | undefined, fallback: string): string {
  const normalized = (value ?? "").trim().toLowerCase();
  if (!normalized) return fallback;
  return normalized.replace(/[^a-z0-9_./:-]+/g, "_").replace(/^_+|_+$/g, "").slice(0, 96) || fallback;
}

function telemetryErrorClass(error: unknown): string {
  if (error instanceof ApiError) return error.code;
  if (error instanceof Error && error.name) return telemetryLabel(error.name, "error");
  return "error";
}

async function sendClientEvent(event: ClientTelemetryEvent): Promise<void> {
  if (!TELEMETRY_ENABLED) return;
  try {
    const rawLaunchParams = await launchParams();
    if (!rawLaunchParams) return;
    await fetch("/miniapp/client-events", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "X-Launch-Params": rawLaunchParams,
      },
      body: JSON.stringify({
        surface: "vk_mini_app",
        event_type: event.event_type,
        screen: telemetryLabel(event.screen, "unknown"),
        route: telemetryRoute(event.route ?? ""),
        status: telemetryLabel(event.status, "unknown"),
        error_class: telemetryLabel(event.error_class, "unknown"),
        step: telemetryLabel(event.step, "unknown"),
        reason: telemetryLabel(event.reason, "unknown"),
        duration_ms:
          typeof event.duration_ms === "number" && Number.isFinite(event.duration_ms)
            ? Math.max(0, Math.min(600_000, Math.round(event.duration_ms)))
            : undefined,
      }),
      keepalive: true,
    });
  } catch {
    /* telemetry must never affect UX */
  }
}

export function installFrontendTelemetry(): void {
  if (!TELEMETRY_ENABLED || telemetryInstalled) return;
  telemetryInstalled = true;
  window.requestAnimationFrame(() => {
    window.setTimeout(() => {
      void sendClientEvent({
        event_type: "ui_event",
        screen: "app",
        step: "launch_rendered",
        reason: "success",
        duration_ms: performance.now() - appStartedAt,
      });
    }, 0);
  });
  window.addEventListener("error", (event) => {
    void sendClientEvent({
      event_type: "js_error",
      screen: "global",
      error_class: event.error?.name ?? "error",
    });
  });
  window.addEventListener("unhandledrejection", (event) => {
    void sendClientEvent({
      event_type: "js_error",
      screen: "global",
      error_class: telemetryErrorClass(event.reason),
    });
  });
}

export function trackPaymentFlowError(step: string, error: unknown): void {
  void sendClientEvent({
    event_type: "payment_flow_error",
    step,
    error_class: telemetryErrorClass(error),
  });
}

export function normalizeRawParams(raw: string): string {
  return raw.replace(/^[?#]/, "");
}

function isForwardedLaunchParamKey(key: string): boolean {
  return key === "sign" || key.startsWith("vk_");
}

function normalizeReferralCode(raw: string | null): string {
  const value = (raw ?? "").trim().toUpperCase();
  if (value.length < 4 || value.length > 64) return "";
  return /^[23456789ABCDEFGHJKLMNPQRSTUVWXYZ_-]+$/.test(value) ? value : "";
}

export function referralCodeFromRaw(raw: string): string {
  const normalized = normalizeRawParams(raw.trim());
  if (!normalized) return "";
  try {
    const params = new URLSearchParams(normalized);
    const direct = normalizeReferralCode(params.get("ref") || params.get("start"));
    if (direct) return direct;
  } catch {
    /* not a query string */
  }
  const queryIndex = normalized.indexOf("?");
  if (queryIndex >= 0) {
    return referralCodeFromRaw(normalized.slice(queryIndex + 1));
  }
  return "";
}

export function referralCodeFromLocation(): string {
  for (const candidate of [window.location.search, window.location.hash]) {
    const code = referralCodeFromRaw(candidate);
    if (code) return code;
  }
  return "";
}

function hasLaunchIdentity(raw: string): boolean {
  const params = new URLSearchParams(normalizeRawParams(raw));
  return Boolean(params.get("vk_user_id"));
}

function sanitizeLaunchParams(raw: string): string {
  const params = new URLSearchParams(normalizeRawParams(raw));
  const out = new URLSearchParams();
  for (const [key, value] of params.entries()) {
    if (!isForwardedLaunchParamKey(key)) continue;
    out.set(key, value);
  }
  const serialized = out.toString();
  return hasLaunchIdentity(serialized) ? serialized : "";
}

export function launchParamsFromLocation(): string {
  const candidates = [window.location.search, window.location.hash];
  for (const candidate of candidates) {
    const raw = normalizeRawParams(candidate);
    const sanitized = sanitizeLaunchParams(raw);
    if (sanitized) return sanitized;

    const queryIndex = raw.indexOf("?");
    if (queryIndex >= 0) {
      const nested = raw.slice(queryIndex + 1);
      const sanitizedNested = sanitizeLaunchParams(nested);
      if (sanitizedNested) return sanitizedNested;
    }
  }
  return "";
}

export function stringifyBridgeLaunchParams(value: unknown): string {
  if (!value || typeof value !== "object") return "";
  const params = new URLSearchParams();
  for (const [key, raw] of Object.entries(value as Record<string, unknown>)) {
    if (!isForwardedLaunchParamKey(key)) continue;
    if (raw === undefined || raw === null) continue;
    if (typeof raw === "boolean") {
      params.set(key, raw ? "1" : "0");
      continue;
    }
    params.set(key, String(raw));
  }
  const out = params.toString();
  return hasLaunchIdentity(out) ? out : "";
}

let launchParamsCache: string | undefined;
let launchParamsInFlight: Promise<string> | undefined;

function bridgeCallTimeoutMs(): number {
  return import.meta.env.DEV ? 1200 : 3000;
}

async function bridgeLaunchParamsFromBridge(): Promise<unknown> {
  const timeoutMs = bridgeCallTimeoutMs();
  let timer: number | undefined;
  const timeout = new Promise<never>((_, reject) => {
    timer = window.setTimeout(() => reject(new Error("vk bridge timeout")), timeoutMs);
  });
  try {
    return await Promise.race([bridge.send("VKWebAppGetLaunchParams"), timeout]);
  } finally {
    if (timer !== undefined) window.clearTimeout(timer);
  }
}

async function resolveLaunchParams(): Promise<string> {
  const fromUrl = launchParamsFromLocation();
  if (fromUrl) {
    return fromUrl;
  }

  try {
    const fromBridge = stringifyBridgeLaunchParams(await bridgeLaunchParamsFromBridge());
    if (fromBridge) {
      return fromBridge;
    }
  } catch {
    /* outside VK, bridge unavailable or timed out */
  }

  const fromDevEnv = import.meta.env.DEV ? import.meta.env.VITE_DEV_LAUNCH_PARAMS : "";
  if (typeof fromDevEnv === "string" && fromDevEnv) {
    return fromDevEnv;
  }

  return "";
}

async function launchParams(): Promise<string> {
  if (launchParamsCache !== undefined) return launchParamsCache;
  if (launchParamsInFlight) return launchParamsInFlight;

  launchParamsInFlight = resolveLaunchParams()
    .then((params) => {
      launchParamsCache = params;
      return params;
    })
    .finally(() => {
      launchParamsInFlight = undefined;
    });
  return launchParamsInFlight;
}

const ARTIFACT_ID_RE =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
const VIDEO_ROUTE_ALIAS_RE = /^video_[a-z0-9_]+$/;

export const MAX_REFERENCE_ARTIFACTS = 4;
export const MAX_UPLOAD_BYTES = 20 << 20;

const ALLOWED_UPLOAD_MIME_TYPES = new Set(["image/jpeg", "image/png"]);

function safeString(value: unknown): string | undefined {
  return typeof value === "string" && value.length > 0 ? value : undefined;
}

function apiErrorCode(status: number, backendError?: string): ApiErrorCode {
  const raw = (backendError ?? "").toLowerCase();
  if (raw === "reference_artifacts_unsupported") {
    return "reference_artifacts_unsupported";
  }
  if (raw === "reference_artifacts_required") {
    return "reference_artifacts_required";
  }
  if (raw === "too_many_reference_artifacts" || raw === "too many reference artifacts") {
    return "too_many_reference_artifacts";
  }
  if (raw === "media_upload_invalid") return "media_upload_invalid";
  if (raw === "media_upload_too_large" || raw === "file too large") return "media_upload_too_large";
  if (raw === "media_upload_unsupported" || raw === "unsupported artifact mime type") {
    return "media_upload_unsupported";
  }
  if (raw === "media_provider_output_invalid") return "media_provider_output_invalid";
  if (raw === "media_processing_unavailable") return "media_processing_unavailable";
  if (raw === "media_delivery_failed") return "media_delivery_failed";
  if (raw === "media_overloaded_retry_later" || raw === "media capacity temporarily unavailable") {
    return "media_overloaded_retry_later";
  }
  if (status === 400 && (raw === "unsupported model" || raw === "unsupported_model")) {
    return "unsupported_model";
  }
  if (status === 400 || raw === "validation_error") return "validation_error";
  if (status === 401 || raw === "auth_error" || raw === "unauthorized") return "auth_error";
  if (status === 402 || raw === "insufficient_credits") return "insufficient_credits";
  if (status === 409 && raw === "payment cannot be canceled") return "payment_cancel_unavailable";
  if (status === 429 || raw === "rate_limited" || raw === "rate limit exceeded") return "rate_limited";
  if (status === 503 || raw === "service_unavailable") return "service_unavailable";
  if (raw === "artifact_unavailable" || raw === "artifact storage unavailable") {
    return "artifact_unavailable";
  }
  return "unknown_error";
}

function apiErrorMessageForCode(code: ApiErrorCode): string {
  switch (code) {
    case "validation_error":
      return "Проверьте запрос и попробуйте снова";
    case "unsupported_model":
      return "Выбранная модель недоступна. Выберите другую модель";
    case "reference_artifacts_required":
      return "Для этой модели нужно загрузить стартовую картинку";
    case "reference_artifacts_unsupported":
      return "Генерация с референсом пока недоступна. Попробуйте без фото или позже";
    case "too_many_reference_artifacts":
      return "Можно добавить не больше 4 референсов";
    case "media_upload_invalid":
      return "Не удалось прочитать файл. Загрузите JPG или PNG";
    case "media_upload_too_large":
      return "Файл слишком большой. Выберите изображение меньшего размера";
    case "media_upload_unsupported":
      return "Формат не поддерживается. Загрузите JPG или PNG";
    case "media_provider_output_invalid":
      return "Медиафайл не прошёл безопасную проверку. ⭐️ не списаны";
    case "media_processing_unavailable":
      return "Медиаобработка временно недоступна. ⭐️ не списаны";
    case "media_delivery_failed":
      return "Не удалось доставить готовый медиафайл. ⭐️ не списаны";
    case "media_overloaded_retry_later":
      return "Сейчас высокая нагрузка на медиаобработку. ⭐️ не списаны, попробуйте позже";
    case "auth_error":
      return "Не удалось подтвердить вход через VK. Откройте приложение заново";
    case "insufficient_credits":
      return "Недостаточно ⭐️";
    case "payment_cancel_unavailable":
      return "Платеж уже нельзя отменить";
    case "rate_limited":
      return "Слишком много запросов. Попробуйте позже";
    case "artifact_unavailable":
    case "service_unavailable":
      return "Сервис временно недоступен";
    case "network_error":
      return "Проблема с сетью. Проверьте подключение";
    default:
      return "Не удалось выполнить запрос";
  }
}

export function apiUserMessage(error: unknown): string {
  if (error instanceof ApiError) return error.userMessage;
  return "Не удалось выполнить запрос";
}

async function apiErrorFromResponse(res: Response): Promise<ApiError> {
  let backendError: string | undefined;
  try {
    const data = await res.json();
    backendError = safeString(data?.error) ?? safeString(data?.message);
  } catch {
    /* ignore */
  }
  return new ApiError(res.status, apiErrorCode(res.status, backendError), {
    backendError,
    retryAfter: res.headers.get("Retry-After") ?? undefined,
  });
}

function validateReferenceArtifactIDs(ids?: string[]): void {
  if (!ids || ids.length === 0) return;
  if (ids.length > MAX_REFERENCE_ARTIFACTS) {
    throw new ApiError(400, "too_many_reference_artifacts", {
      backendError: "too many reference artifacts",
    });
  }
  for (const id of ids) {
    if (!ARTIFACT_ID_RE.test(id)) {
      throw new ApiError(400, "validation_error", {
        backendError: "invalid reference artifact id",
      });
    }
  }
}

function validateUploadFile(file: File): void {
  if (!ALLOWED_UPLOAD_MIME_TYPES.has(file.type)) {
    throw new ApiError(400, "validation_error", {
      backendError: "unsupported artifact mime type",
    });
  }
  if (file.size > MAX_UPLOAD_BYTES) {
    throw new ApiError(400, "validation_error", {
      backendError: "artifact too large",
    });
  }
}

export function createIdempotencyKey(): string {
  if (globalThis.crypto?.randomUUID) {
    return globalThis.crypto.randomUUID();
  }
  if (globalThis.crypto?.getRandomValues) {
    const bytes = new Uint8Array(16);
    globalThis.crypto.getRandomValues(bytes);
    return Array.from(bytes, (b) => b.toString(16).padStart(2, "0")).join("");
  }
  return `ui-${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}`;
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  let res: Response;
  const started = performance.now();
  try {
    const rawLaunchParams = await launchParams();
    res = await fetch(path, {
      ...init,
      headers: {
        "Content-Type": "application/json",
        "X-Launch-Params": rawLaunchParams,
        ...(init?.headers ?? {}),
      },
    });
  } catch {
    const durationMs = performance.now() - started;
    void sendClientEvent({
      event_type: "api_failure",
      route: path,
      status: "network",
      error_class: "network_error",
      duration_ms: durationMs,
    });
    throw new ApiError(0, "network_error");
  }
  const durationMs = performance.now() - started;
  void sendClientEvent({
    event_type: "api_latency",
    route: path,
    status: String(res.status),
    duration_ms: durationMs,
  });
  if (!res.ok) {
    const err = await apiErrorFromResponse(res);
    void sendClientEvent({
      event_type: "api_failure",
      route: path,
      status: String(res.status),
      error_class: err.code,
      duration_ms: durationMs,
    });
    throw err;
  }
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

export async function getBalance(): Promise<number> {
  const data = await request<BalanceResponse>("/miniapp/balance");
  return data.balance_credits ?? 0;
}

export async function getReferral(): Promise<ReferralInfo> {
  return request<ReferralInfo>("/miniapp/referral");
}

export async function acceptReferral(code: string): Promise<ApplyReferralResponse> {
  return request<ApplyReferralResponse>("/miniapp/referral/accept", {
    method: "POST",
    body: JSON.stringify({ code }),
  });
}

export async function listPaymentProducts(): Promise<PaymentProduct[]> {
  const data = await request<PaymentProductListResponse>("/miniapp/payment-products");
  return data.items ?? [];
}

export async function createPaymentIntent(
  input: CreatePaymentIntentInput,
  options: CreateJobOptions,
): Promise<PaymentIntent> {
  try {
    return await request<PaymentIntent>("/miniapp/payments/intents", {
      method: "POST",
      headers: {
        "X-Idempotency-Key": options.idempotencyKey,
      },
      body: JSON.stringify(input),
    });
  } catch (error) {
    trackPaymentFlowError("create_intent", error);
    throw error;
  }
}

export async function listPaymentIntents(): Promise<PaymentIntent[]> {
  const data = await request<PaymentIntentListResponse>("/miniapp/payments");
  return data.items ?? [];
}

function validateVideoRouteAlias(alias?: string): void {
  if (!alias) return;
  if (!VIDEO_ROUTE_ALIAS_RE.test(alias)) {
    throw new ApiError(400, "validation_error", {
      backendError: "invalid video route alias",
    });
  }
}

export async function cancelPaymentIntent(id: string): Promise<PaymentIntent> {
  try {
    return await request<PaymentIntent>(`/miniapp/payments/${encodeURIComponent(id)}/cancel`, {
      method: "POST",
    });
  } catch (error) {
    trackPaymentFlowError("cancel_intent", error);
    throw error;
  }
}

export async function listJobs(): Promise<Job[]> {
  const data = await request<JobListResponse>("/miniapp/jobs");
  return data.items ?? [];
}

interface ModelCatalogListResponse {
  items: ModelCatalogItem[];
}

export async function listModelCatalog(): Promise<ModelCatalogItem[]> {
  const data = await request<ModelCatalogListResponse>("/miniapp/model-catalog");
  return data.items ?? [];
}

export async function getJob(id: string): Promise<Job> {
  return request<Job>(`/miniapp/jobs/${id}`);
}

export async function createJob(input: CreateJobInput, options: CreateJobOptions): Promise<Job> {
  validateReferenceArtifactIDs(input.reference_artifact_ids);
  validateVideoRouteAlias(input.video_route_alias);
  return request<Job>("/miniapp/jobs", {
    method: "POST",
    headers: {
      "X-Idempotency-Key": options.idempotencyKey,
    },
    body: serializeGenerationRequest(input),
  });
}

export async function uploadArtifact(file: File): Promise<string> {
  validateUploadFile(file);

  const body = new FormData();
  body.append("file", file);

  let res: Response;
  try {
    const rawLaunchParams = await launchParams();
    res = await fetch("/miniapp/artifacts", {
      method: "POST",
      headers: {
        "X-Launch-Params": rawLaunchParams,
        "X-Idempotency-Key": createIdempotencyKey(),
      },
      body,
    });
  } catch {
    throw new ApiError(0, "network_error");
  }

  if (!res.ok) {
    throw await apiErrorFromResponse(res);
  }

  const data = (await res.json()) as ArtifactUploadResponse;
  if (!ARTIFACT_ID_RE.test(data.artifact_id)) {
    throw new ApiError(500, "service_unavailable", {
      backendError: "invalid artifact response",
    });
  }
  return data.artifact_id;
}

export async function createChatMessage(input: CreateChatMessageInput, options: CreateJobOptions): Promise<Job> {
  return request<Job>("/miniapp/chat/messages", {
    method: "POST",
    headers: {
      "X-Idempotency-Key": options.idempotencyKey,
    },
    body: JSON.stringify(input),
  });
}

export async function listChatConversations(): Promise<ChatConversation[]> {
  const data = await request<ChatConversationListResponse>("/miniapp/chat/conversations");
  return data.items ?? [];
}

export async function listChatConversationMessages(conversationId: string): Promise<ChatConversationMessage[]> {
  const data = await request<ChatConversationMessageListResponse>(
    `/miniapp/chat/conversations/${encodeURIComponent(conversationId)}/messages`,
  );
  return data.items ?? [];
}

export async function estimateJob(input: EstimateInput): Promise<EstimateResponse> {
  validateReferenceArtifactIDs(input.reference_artifact_ids);
  validateVideoRouteAlias(input.video_route_alias);
  return request<EstimateResponse>("/miniapp/estimate", {
    method: "POST",
    body: serializeGenerationRequest(input),
  });
}

function serializeGenerationRequest(input: CreateJobInput | EstimateInput): string {
  return JSON.stringify({
    operation: input.operation,
    prompt: input.prompt,
    model_id: input.model_id,
    video_route_alias: input.video_route_alias,
    image_quality: input.image_quality,
    reference_artifact_ids: input.reference_artifact_ids,
    duration_sec: input.duration_sec,
  });
}

/** Only trusted artifact UUIDs from job DTO — never arbitrary URLs. */
export function artifactUrl(id: string): string | null {
  if (!ARTIFACT_ID_RE.test(id)) return null;
  return `/miniapp/artifacts/${id}`;
}

async function fetchArtifactBlob(id: string): Promise<Blob | null> {
  const url = artifactUrl(id);
  if (!url) return null;
  try {
    const rawLaunchParams = await launchParams();
    const res = await fetch(url, {
      headers: { "X-Launch-Params": rawLaunchParams },
    });
    if (!res.ok) return null;
    return await res.blob();
  } catch {
    return null;
  }
}

/**
 * Authenticated artifact source for <img>/<video>. Browser media tags cannot
 * send headers, so the frontend fetches with X-Launch-Params and exposes only
 * a temporary blob URL. Never put raw launch params into media src/query URLs.
 */
export async function artifactMediaUrl(id: string): Promise<string | null> {
  const blob = await fetchArtifactBlob(id);
  return blob ? URL.createObjectURL(blob) : null;
}

/** Fetch artifact bytes and return a blob URL for instant preview on the result screen. */
export async function preloadArtifactBlobUrl(id: string): Promise<string | null> {
  return artifactMediaUrl(id);
}

/** Text artifact body (when GET /miniapp/artifacts/{id} is available). */
export async function fetchArtifactText(id: string): Promise<string | null> {
  const url = artifactUrl(id);
  if (!url) return null;
  const rawLaunchParams = await launchParams();
  const res = await fetch(url, {
    headers: { "X-Launch-Params": rawLaunchParams },
  });
  if (!res.ok) return null;
  const ct = res.headers.get("content-type") ?? "";
  if (!ct.includes("text")) return null;
  const text = await res.text();
  return text.length > 100_000 ? text.slice(0, 100_000) : text;
}

const OK = new Set(["succeeded"]);
const FAIL = new Set(["failed_terminal", "rejected", "cancelled", "expired", "refunded"]);

export type StatusKind = "done" | "failed" | "progress";

export function statusKind(s: string): StatusKind {
  if (OK.has(s)) return "done";
  if (FAIL.has(s)) return "failed";
  return "progress";
}

export function isTerminal(s: string): boolean {
  return OK.has(s) || FAIL.has(s);
}

/** Image/video artifacts are previewable only after backend marks them fully visible. */
export function hasPreviewableMediaResult(job: Job): boolean {
  if (!job.output_artifact_ids?.length) return false;
  if (job.operation !== "image_generate" && job.operation !== "video_generate") return false;
  return statusKind(job.status) === "done";
}

const STATUS_LABELS: Record<string, string> = {
  received: "Принято",
  validated: "Проверка",
  credits_reserved: "Резерв ⭐️",
  awaiting_payment: "Ожидает оплаты",
  queued: "В очереди",
  dispatching_provider: "Отправка",
  provider_submitted: "Передано",
  provider_pending: "В очереди у провайдера",
  provider_processing: "Генерация…",
  provider_succeeded: "Готовится",
  postprocessing: "Постобработка",
  result_ready: "Почти готово",
  delivering: "Доставка",
  succeeded: "Готово",
};

export function statusLabel(s: string): string {
  if (statusKind(s) === "done") return STATUS_LABELS.succeeded;
  return STATUS_LABELS[s] ?? "Обработка…";
}

const ERROR_LABELS: Record<string, string> = {
  insufficient_credits: "Недостаточно ⭐️",
  provider_error: "Временная ошибка генерации",
  timeout: "Превышено время ожидания",
  rate_limited: "Слишком много запросов",
  media_upload_invalid: "Не удалось прочитать файл",
  media_upload_too_large: "Файл слишком большой",
  media_upload_unsupported: "Формат файла не поддерживается",
  media_provider_output_invalid: "Медиафайл не прошёл безопасную проверку. ⭐️ не списаны",
  media_processing_unavailable: "Медиаобработка временно недоступна. ⭐️ не списаны",
  media_delivery_failed: "Не удалось доставить готовый медиафайл. ⭐️ не списаны",
  media_overloaded_retry_later: "Высокая нагрузка на медиаобработку. ⭐️ не списаны",
};

export function errorLabel(job: Job): string {
  if (job.error_code && ERROR_LABELS[job.error_code]) return ERROR_LABELS[job.error_code];
  if (job.status === "expired") return "Истёк срок";
  if (job.status === "cancelled") return "Отменено";
  return "Не удалось выполнить запрос";
}

/** JobDTO has no inline text; load from text artifact when available. */
export function botText(_job: Job): string | null {
  return null;
}

export async function resolveBotText(job: Job): Promise<string | undefined> {
  const inline = botText(job);
  if (inline) return inline;
  if (job.operation === "text_generate" && job.output_artifact_ids.length > 0) {
    const text = await fetchArtifactText(job.output_artifact_ids[0]);
    if (text) return text;
  }
  return undefined;
}
