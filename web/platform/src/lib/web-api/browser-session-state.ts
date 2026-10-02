import { ReadError } from "./read-error";

let accountId: string | null = null;
let revision = 0;
let controller = new AbortController();

export function beginBrowserSession(id: string) {
  if (accountId !== id) { endBrowserSession(); accountId = id; }
}
export function endBrowserSession() {
  revision++; accountId = null; controller.abort(); controller = new AbortController();
}
export function browserSessionSnapshot() { return { accountId, revision, signal: controller.signal }; }
export function assertBrowserSession(response: Response, expected: ReturnType<typeof browserSessionSnapshot>) {
  if (expected.revision !== revision) throw new DOMException("Request cancelled.", "AbortError");
  const returnedAccount = response.headers.get("X-NeiroHub-Account-ID");
  if (expected.accountId && returnedAccount && expected.accountId !== returnedAccount) {
    window.dispatchEvent(new Event("neirohub:account-changed"));
    throw new ReadError("session", 401);
  }
}
