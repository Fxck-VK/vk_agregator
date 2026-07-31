import { getWebApiInternalOrigin } from "../../../../lib/web-api/internal-origin";
import { proxyWebApiRequest } from "../../../../lib/web-api/proxy";

export const runtime = "nodejs";

async function handle(request: Request): Promise<Response> {
  const requestURL = new URL(request.url);
  const rawPath = `${requestURL.pathname}${requestURL.search}`;
  return proxyWebApiRequest(request, rawPath, getWebApiInternalOrigin());
}

export {
  handle as DELETE,
  handle as GET,
  handle as HEAD,
  handle as OPTIONS,
  handle as PATCH,
  handle as POST,
  handle as PUT,
};
