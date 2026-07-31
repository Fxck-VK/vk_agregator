import "server-only";

const invalidInternalOriginMessage = "WEB_API_INTERNAL_ORIGIN must be a plain HTTP(S) origin.";
const plainHttpOrigin = /^https?:\/\/[^/?#]+\/?$/i;

export function getWebApiInternalOrigin(): string {
  const value = process.env.WEB_API_INTERNAL_ORIGIN;
  if (!value || !plainHttpOrigin.test(value)) {
    throw new Error(invalidInternalOriginMessage);
  }

  let origin: URL;
  try {
    origin = new URL(value);
  } catch {
    throw new Error(invalidInternalOriginMessage);
  }

  if (
    (origin.protocol !== "http:" && origin.protocol !== "https:") ||
    origin.pathname !== "/" ||
    origin.search ||
    origin.hash ||
    origin.username ||
    origin.password
  ) {
    throw new Error(invalidInternalOriginMessage);
  }

  return origin.origin;
}
