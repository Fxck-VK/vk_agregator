import { readFile } from "node:fs/promises";
import { join } from "node:path";

import {
  isLocalWorkspacePreviewEnabled,
  localWorkspacePreviewChatModels,
  localWorkspacePreviewModels,
  localWorkspacePreviewVideoModels,
  localWorkspacePreviewConversationMessages,
  localWorkspacePreviewImageArtifactPaths,
  localWorkspacePreviewImageJobResults,
  localWorkspacePreviewImageJobs,
  localWorkspacePreviewImageModels,
  localWorkspacePreviewPaymentProducts,
} from "../../../../features/session/local-workspace-preview";
import { getWebApiInternalOrigin } from "../../../../lib/web-api/internal-origin";
import { proxyWebApiRequest } from "../../../../lib/web-api/proxy";

export const runtime = "nodejs";

async function handle(request: Request): Promise<Response> {
  const requestURL = new URL(request.url);
  const rawPath = `${requestURL.pathname}${requestURL.search}`;
  if (isLocalWorkspacePreviewEnabled() && requestURL.pathname.startsWith("/web/v1/payments/")) {
    return Response.json({ error: "Payments require a real authenticated backend" }, { status: 503, headers: { "Cache-Control": "no-store" } });
  }
  if (isLocalWorkspacePreviewEnabled() && request.method === "GET") {
    if (requestURL.pathname === "/web/v1/payment-products") return previewJson(localWorkspacePreviewPaymentProducts);
    if (rawPath === "/web/v1/models") return previewJson(localWorkspacePreviewModels);
    if (rawPath === "/web/v1/video-models") return previewJson(localWorkspacePreviewVideoModels);
    if (rawPath === "/web/v1/chat-models") {
      return previewJson(localWorkspacePreviewChatModels);
    }
    if (rawPath === "/web/v1/image-models") {
      return previewJson(localWorkspacePreviewImageModels);
    }
    if (requestURL.pathname === "/web/v1/image-jobs") {
      return previewJson(localWorkspacePreviewImageJobs);
    }
    if (requestURL.pathname === "/web/v1/music-jobs") {
      return previewJson({ items: [], has_more: false, next_cursor: null });
    }

    const conversationMessagesMatch = requestURL.pathname.match(
      /^\/web\/v1\/conversations\/([^/]+)\/messages$/,
    );
    const conversationMessages = conversationMessagesMatch === null
      ? undefined
      : localWorkspacePreviewConversationMessages[conversationMessagesMatch[1]];
    if (conversationMessages !== undefined) {
      return previewJson(conversationMessages);
    }

    const resultMatch = requestURL.pathname.match(/^\/web\/v1\/image-jobs\/([^/]+)\/result$/);
    const result = resultMatch === null ? undefined : localWorkspacePreviewImageJobResults[resultMatch[1]];
    if (result !== undefined) {
      return previewJson(result);
    }

    const artifactMatch = requestURL.pathname.match(/^\/web\/v1\/image-artifacts\/([^/]+)$/);
    const artifactPath = artifactMatch === null ? undefined : localWorkspacePreviewImageArtifactPaths[artifactMatch[1]];
    if (artifactPath !== undefined) {
      const artifact = await readFile(join(process.cwd(), "public", artifactPath.slice(1)));
      return new Response(new Uint8Array(artifact), {
        headers: {
          "Cache-Control": "no-store",
          "Content-Type": "image/png",
        },
      });
    }
  }
  return proxyWebApiRequest(request, rawPath, getWebApiInternalOrigin());
}

function previewJson(payload: unknown): Response {
  return Response.json(payload, {
    headers: { "Cache-Control": "no-store" },
  });
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
