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
  const url = new URL(request.url);
  if (request.method === "GET" && url.searchParams.get("preview") === "1" && /^\/web\/v1\/image-artifacts\/[0-9a-f-]{36}$/i.test(url.pathname)) {
    // Reuse normal authorization. The browser cannot supply an upstream URL.
    url.search = "";
    const headers = new Headers(request.headers);
    headers.delete("Range"); headers.delete("If-Range");
    const source = await handleSource(new Request(url, { headers, signal: request.signal }));
    const { imagePreviewResponse } = await import("@/features/files/image-preview.server");
    return imagePreviewResponse(source, request.signal);
  }
  return handleSource(request);
}

async function handleSource(request: Request): Promise<Response> {
  const requestURL = new URL(request.url);
  const rawPath = `${requestURL.pathname}${requestURL.search}`;
  if (isLocalWorkspacePreviewEnabled() && !["GET", "HEAD", "OPTIONS"].includes(request.method)) {
    return Response.json({ error: "Local preview does not forward writes to a backend" }, { status: 503, headers: { "Cache-Control": "no-store" } });
  }
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
    if (artifactMatch?.[1].startsWith("f1000000-")) {
      const { previewImageResponse } = await import("@/features/local-development/preview-image.server");
      return await previewImageResponse(artifactMatch[1]) ?? Response.json({ error: "Invalid preview image" }, { status: 404 });
    }
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
  if (isLocalWorkspacePreviewEnabled()) {
    return Response.json({ error: "This endpoint is not available in local preview" }, { status: 503, headers: { "Cache-Control": "no-store" } });
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
