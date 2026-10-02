import "server-only";

import { createHash, createHmac, randomBytes, timingSafeEqual } from "node:crypto";

const cookieName = "__Host-nh-dev-access";
const lifetimeSeconds = 30 * 24 * 60 * 60;
const devOrigin = "https://dev-web.neiirohub.ru";
const credentialPattern = /^[A-Za-z0-9][A-Za-z0-9._-]{0,63}:(\$2[aby]\$[0-9]{2}\$[./A-Za-z0-9]{53}|\$apr1\$[./A-Za-z0-9]{1,8}\$[./A-Za-z0-9]{22})$/;

type DevAccessConfig = { username: string; signingKey: Buffer; issuerProof: string };

// This is a separate outer DEV gate, never an account session or identity.
// The existing htpasswd entry is a deployment secret, not a public hash. Domain
// separation keeps the issuer proof independent from the cookie-signing key.
// Keeping the entry stable preserves sessions; rotating it revokes all of them.
export function getDevAccessConfig(): DevAccessConfig | null {
  const credential = process.env.DEV_WEB_BASIC_AUTH_HTPASSWD ?? "";
  if (process.env.WEB_ORIGIN !== devOrigin || !credentialPattern.test(credential)) return null;
  return {
    username: credential.slice(0, credential.indexOf(":")),
    signingKey: createHash("sha256").update(`neirohub-dev-web-session-v1:${credential}`).digest(),
    issuerProof: createHash("sha256").update(`neirohub-dev-web-issuer-v1:${credential}`).digest("hex"),
  };
}

function constantTimeEqual(left: string, right: string): boolean {
  const a = Buffer.from(left);
  const b = Buffer.from(right);
  return a.length === b.length && timingSafeEqual(a, b);
}

export function isDevAccessIssuer(config: DevAccessConfig, proof: string | null, username: string | null): boolean {
  return username === config.username && proof !== null && proof.length === 64
    && constantTimeEqual(proof, config.issuerProof);
}

function signature(config: DevAccessConfig, payload: string): string {
  return createHmac("sha256", config.signingKey).update(payload).digest("base64url");
}

export function issueDevAccessCookie(config: DevAccessConfig, now = new Date()): string {
  const issuedAt = Math.floor(now.getTime() / 1000);
  const expiresAt = issuedAt + lifetimeSeconds;
  const payload = `v1.${issuedAt}.${expiresAt}.${randomBytes(24).toString("base64url")}`;
  return `${cookieName}=${payload}.${signature(config, payload)}; Path=/; Max-Age=${lifetimeSeconds}; Expires=${new Date(expiresAt * 1000).toUTCString()}; HttpOnly; Secure; SameSite=Lax`;
}

export function hasDevAccess(config: DevAccessConfig, cookies: string | null, now = new Date()): boolean {
  const values = (cookies ?? "").split(";").map(part => part.trim()).filter(part => part.startsWith(`${cookieName}=`));
  if (values.length !== 1) return false;
  const token = values[0].slice(cookieName.length + 1);
  const parts = /^(v1\.(\d{10})\.(\d{10})\.[A-Za-z0-9_-]{32})\.([A-Za-z0-9_-]{43})$/.exec(token);
  if (!parts || !constantTimeEqual(signature(config, parts[1]), parts[4])) return false;
  const issuedAt = Number(parts[2]);
  const expiresAt = Number(parts[3]);
  const current = Math.floor(now.getTime() / 1000);
  return expiresAt - issuedAt === lifetimeSeconds && issuedAt <= current && current < expiresAt;
}

export function safeDevReturnPath(value: string | null): string {
  if (!value || value.length > 2048 || !value.startsWith("/") || value.startsWith("//")
    || /[\\\u0000-\u0020\u007f]/.test(value) || /%(?:2f|5c|25|0[ad])/i.test(value)) return "/";
  try {
    const target = new URL(value, devOrigin);
    const path = decodeURIComponent(target.pathname).replace(/^\/(?:ru|en)(?=\/|$)/, "");
    if (target.origin !== devOrigin || /^\/(?:__dev|web|api|_next|assets|health)(?:\/|$)/.test(path)) return "/";
    return target.pathname + target.search;
  } catch {
    return "/";
  }
}
