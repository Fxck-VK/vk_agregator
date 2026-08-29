import {
  isLocalWorkspacePreviewEnabled,
  localWorkspacePreviewImageModels,
} from "../../../../features/session/local-workspace-preview";
import { getWebApiInternalOrigin } from "../../../../lib/web-api/internal-origin";
import { proxyWebApiRequest } from "../../../../lib/web-api/proxy";

export const runtime = "nodejs";

async function handle(request: Request): Promise<Response> {
  const requestURL = new URL(request.url);
  const rawPath = `${requestURL.pathname}${requestURL.search}`;
  if (
    isLocalWorkspacePreviewEnabled() &&
    request.method === "GET" &&
    rawPath === "/web/v1/image-models"
  ) {
    return Response.json(localWorkspacePreviewImageModels, {
      headers: { "Cache-Control": "no-store" },
    });
  }
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
