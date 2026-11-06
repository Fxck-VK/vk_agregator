import { releaseStateForRequest } from "@/features/release/server";
import { releaseViewCookie } from "@/features/release/policy";
const response = (status: number, body: unknown = null) => Response.json(body, { status, headers: { "Cache-Control": "private, no-store", "X-Robots-Tag": "noindex" } });
export async function GET(request: Request) {
  const state = releaseStateForRequest(request.headers);
  return state.canSwitch ? response(200, state) : response(404);
}
export async function POST(request: Request) {
  const state = releaseStateForRequest(request.headers);
  if (!state.canSwitch) return response(404);
  const origin = state.localSimulation ? "http://localhost:7159" : "https://dev-web.epifera.com";
  if (request.headers.get("origin") !== origin) return response(403);
  if (request.headers.get("content-type")?.split(";", 1)[0].trim().toLowerCase() !== "application/json") return response(400);
  const reader = request.body?.getReader();
  if (!reader) return response(400);
  let size = 0; const chunks: Uint8Array[] = [];
  try {
    while (true) { const part = await reader.read(); if (part.done) break; size += part.value.length; if (size > 2048) { await reader.cancel(); return response(413); } chunks.push(part.value); }
    const bytes = new Uint8Array(size); let offset = 0;
    for (const chunk of chunks) { bytes.set(chunk, offset); offset += chunk.length; }
    const value: unknown = JSON.parse(new TextDecoder("utf-8", { fatal: true }).decode(bytes));
    if (!value || typeof value !== "object" || Array.isArray(value) || Object.keys(value).length !== 1 || !("view" in value) || (value.view !== "dev" && value.view !== "prod")) return response(400);
    const result = response(200, { ...state, view: value.view });
    result.headers.append("Set-Cookie", `${releaseViewCookie}=${value.view}; Path=/; Max-Age=604800; HttpOnly; SameSite=Lax${state.localSimulation ? "" : "; Secure"}`);
    return result;
  } catch { return response(400); }
}
