import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
vi.mock("./sign-in-navigation", () => ({ replaceSignInDocument: vi.fn() }));
import { replaceSignInDocument } from "./sign-in-navigation";
import { classifySignInFailure, completeSignIn, reportSignInFailure, signInResponseFailure } from "./sign-in-completion";

const account = "10000000-0000-4000-8000-000000000001";
const requestId = "40000000-0000-4000-8000-000000000001";
const controller = () => new AbortController();
const accepted = () => new Response(null, { status: 201, headers: { "X-NeiroHub-Account-ID": account } });
const current = (id = account) => Response.json({ account_id: id, identity_refs: [], password_set: true }, { headers: { "X-Request-ID": requestId } });
beforeEach(() => { vi.stubGlobal("fetch", vi.fn().mockImplementation(async () => current())); });
afterEach(() => { vi.useRealTimers(); vi.restoreAllMocks(); vi.clearAllMocks(); vi.unstubAllGlobals(); });

describe("sign-in completion", () => {
  it("requires fresh cookie proof and performs full document navigation to a safe localized URL", async () => {
    const abort = controller(); const stage = vi.fn();
    const completion = completeSignIn(accepted(), { locale: "en", returnTo: "/ru/app/files", signal: abort.signal, onStage: stage });
    const rejection = expect(completion).rejects.toMatchObject({ name: "AbortError" });
    await vi.waitFor(() => expect(replaceSignInDocument).toHaveBeenCalledWith("/en/app/files"));
    expect(fetch).toHaveBeenCalledWith("/web/v1/me", expect.objectContaining({ credentials: "include", cache: "no-store", signal: expect.any(AbortSignal) }));
    expect(stage.mock.calls).toEqual([["session"], ["navigation"]]);
    abort.abort(); await rejection;
  });
  it.each([401, 503, 429, 504])("does not navigate after session confirmation HTTP %s", async status => {
    vi.mocked(fetch).mockResolvedValue(new Response(null, { status, headers: { "X-Request-ID": requestId } }));
    await expect(completeSignIn(accepted(), { locale: "ru", signal: controller().signal, onStage: vi.fn() })).rejects.toMatchObject({ stage: "session", status, requestId });
    expect(replaceSignInDocument).not.toHaveBeenCalled();
  });
  it.each([{}, { account_id: account, identity_refs: "invalid" }, { account_id: account, identity_refs: [], token: "synthetic-secret" }])("rejects malformed or unsafe session payloads", async payload => {
    vi.mocked(fetch).mockResolvedValue(Response.json(payload));
    await expect(completeSignIn(accepted(), { locale: "ru", signal: controller().signal, onStage: vi.fn() })).rejects.toMatchObject({ stage: "session", code: "invalid_session" });
    expect(replaceSignInDocument).not.toHaveBeenCalled();
  });
  it("rejects an account changed by another tab before confirmation", async () => {
    vi.mocked(fetch).mockResolvedValue(current("10000000-0000-4000-8000-000000000002"));
    await expect(completeSignIn(accepted(), { locale: "ru", signal: controller().signal, onStage: vi.fn() })).rejects.toMatchObject({ code: "account_changed" });
    expect(replaceSignInDocument).not.toHaveBeenCalled();
  });
  it("bounds a session request that never receives a response", async () => {
    const deadline = controller();
    vi.spyOn(AbortSignal, "timeout").mockReturnValue(deadline.signal);
    vi.mocked(fetch).mockImplementation((_path, options) => new Promise((_resolve, reject) => {
      options?.signal?.addEventListener("abort", () => reject(options.signal?.reason));
    }));
    const completion = completeSignIn(accepted(), { locale: "ru", signal: controller().signal, onStage: vi.fn() });
    const rejection = expect(completion).rejects.toMatchObject({ stage: "session", code: "timeout" });
    deadline.abort(new DOMException("", "TimeoutError"));
    await rejection;
    expect(replaceSignInDocument).not.toHaveBeenCalled();
  });
  it("recovers from a navigation that never unloads the login page", async () => {
    vi.useFakeTimers();
    const completion = completeSignIn(accepted(), { locale: "ru", returnTo: "https://external.example/", signal: controller().signal, onStage: vi.fn() });
    const rejection = expect(completion).rejects.toMatchObject({ stage: "navigation", code: "navigation_failed" });
    await vi.advanceTimersByTimeAsync(12_001);
    await rejection;
    expect(replaceSignInDocument).toHaveBeenCalledWith("/ru/app");
  });
  it("logs only enumerated metadata, rejecting arbitrary response headers and raw error messages", () => {
    const log = vi.spyOn(console, "warn").mockImplementation(() => {});
    const failure = signInResponseFailure(new Response("synthetic-password", { status: 503, headers: { "X-Request-ID": "email@example.test" } }), "credentials");
    reportSignInFailure(failure, performance.now());
    const detail = log.mock.calls[0][1];
    expect(detail).toEqual({ diagnostic_id: expect.stringMatching(/^[0-9a-f-]{36}$/), request_id: null, code: "unavailable", stage: "credentials", status: 503, duration_ms: expect.any(Number) });
    expect(JSON.stringify(log.mock.calls)).not.toMatch(/email@example|synthetic-password/);
    expect(classifySignInFailure(new Error("synthetic-private-detail"), "credentials").message).not.toContain("synthetic-private-detail");
  });
});
