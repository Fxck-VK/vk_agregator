import type { Locale } from "@/i18n/locales";
import type { accountAuthText } from "@/i18n/account-auth";
import { localizeHref } from "@/i18n/routing";
import { accountProfileSchema } from "@/lib/web-api/contracts";
import { safeReturnPath } from "./return-path";
import { replaceSignInDocument } from "./sign-in-navigation";

export type SignInStage = "credentials" | "session" | "navigation";
export type SignInCode = "credentials_invalid" | "rate_limited" | "unavailable" | "timeout" | "network" | "dev_access" | "request_rejected" | "session_unconfirmed" | "invalid_session" | "account_changed" | "navigation_failed";
const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
function safeRequestId(response?: Response): string | undefined {
  const value = response?.headers.get("X-Request-ID");
  return value && uuid.test(value) ? value : undefined;
}

/** Contains only allowlisted metadata, never an error cause or response body. */
export class SignInFailure extends Error {
  readonly diagnosticId: string;
  constructor(readonly stage: SignInStage, readonly code: SignInCode, readonly status?: number, readonly requestId?: string) {
    super("Sign-in could not be completed.");
    this.name = "SignInFailure";
    this.diagnosticId = requestId ?? crypto.randomUUID();
  }
}

export function signInResponseFailure(response: Response, stage: SignInStage): SignInFailure {
  const status = response.status;
  const code = response.headers.get("X-NeiroHub-Dev-Access") === "required" ? "dev_access"
    : status === 401 ? (stage === "credentials" ? "credentials_invalid" : "session_unconfirmed")
    : status === 429 ? "rate_limited" : status === 408 || status === 504 ? "timeout"
    : status >= 500 ? "unavailable" : "request_rejected";
  return new SignInFailure(stage, code, status, safeRequestId(response));
}

export function classifySignInFailure(value: unknown, stage: SignInStage): SignInFailure {
  if (value instanceof SignInFailure) return value;
  const name = typeof value === "object" && value !== null && "name" in value ? value.name : "";
  return new SignInFailure(stage, name === "TimeoutError" || name === "AbortError" ? "timeout" : "network");
}

export function reportSignInFailure(failure: SignInFailure, started: number): void {
  console.warn("auth_sign_in_failed", {
    diagnostic_id: failure.diagnosticId, request_id: failure.requestId ?? null,
    stage: failure.stage, code: failure.code, status: failure.status ?? null,
    duration_ms: Math.max(0, Math.round(performance.now() - started)),
  });
}

type Text = { [K in keyof typeof accountAuthText.ru]: string };
export function signInFailureText(failure: SignInFailure, text: Text, invalidCredentials: string): string {
  const messages: Record<SignInCode, string> = {
    credentials_invalid: invalidCredentials, rate_limited: text.registrationRateLimited,
    unavailable: text.loginUnavailable, timeout: text.loginTimeout, network: text.loginNetwork,
    dev_access: text.loginDevAccess, request_rejected: text.loginRejected,
    session_unconfirmed: text.loginSessionUnconfirmed, invalid_session: text.loginSessionUnconfirmed,
    account_changed: text.loginAccountChanged, navigation_failed: text.loginNavigationFailed,
  };
  return messages[failure.code];
}

export async function completeSignIn(response: Response, options: {
  locale: Locale; returnTo?: string; signal: AbortSignal;
  onStage: (stage: "session" | "navigation") => void;
}): Promise<never> {
  options.signal.throwIfAborted();
  options.onStage("session");
  let current: Response | undefined;
  try {
    // Verify cookies explicitly; general read recovery could conceal this failure
    // or redirect before the login form can explain it.
    current = await fetch("/web/v1/me", { credentials: "include", cache: "no-store", headers: { Accept: "application/json" }, signal: AbortSignal.any([options.signal, AbortSignal.timeout(10_000)]) });
    if (!current.ok) throw signInResponseFailure(current, "session");
    const profile = accountProfileSchema.safeParse(await current.json());
    if (!profile.success) throw new SignInFailure("session", "invalid_session", current.status, safeRequestId(current));
    const expectedAccount = response.headers.get("X-NeiroHub-Account-ID");
    if (expectedAccount && (!uuid.test(expectedAccount) || profile.data.account_id !== expectedAccount)) {
      throw new SignInFailure("session", "account_changed", current.status, safeRequestId(current));
    }
  } catch (value) {
    if (options.signal.aborted) throw options.signal.reason;
    if (value instanceof SyntaxError) throw new SignInFailure("session", "invalid_session", current?.status, safeRequestId(current));
    throw classifySignInFailure(value, "session");
  }
  options.signal.throwIfAborted();
  options.onStage("navigation");
  const target = localizeHref(safeReturnPath(options.returnTo ?? "") ?? "/app", options.locale);
  try { replaceSignInDocument(target); }
  catch { throw new SignInFailure("navigation", "navigation_failed", undefined, safeRequestId(current)); }
  // Successful navigation unloads the form. If it does not, recover its controls
  // with an explicit error instead of leaving an endless spinner.
  return new Promise<never>((_resolve, reject) => {
    const abort = () => { clearTimeout(timer); reject(options.signal.reason); };
    const timer = setTimeout(() => {
      options.signal.removeEventListener("abort", abort);
      reject(new SignInFailure("navigation", "navigation_failed", undefined, safeRequestId(current)));
    }, 12_000);
    options.signal.addEventListener("abort", abort, { once: true });
    if (options.signal.aborted) abort();
  });
}
