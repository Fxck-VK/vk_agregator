"use client";

import { useMessages } from "@/i18n/LocaleProvider";


import { useEffect, useState } from "react";
import { webBrowserFetch } from "@/lib/web-api/browser";
import type { GenerationOptions } from "@/features/models/generation-options-contract";
import type { GenerationModel } from "@/features/models/generation-model-catalog";

type QuoteAttachments = { count: number; ready: boolean; artifactIds: readonly string[] };
type QuoteRequest = { key: string; attempt: number };

// Reference pricing is resolved by the same backend resolver as the job.
export function useReferenceQuote(model: GenerationModel | undefined, options: GenerationOptions, attachments: QuoteAttachments) {
  const msg = useMessages();
  const { count } = attachments;
  const query = model?.category === "images" && count > 0 ? new URLSearchParams({ model_id: model.id, image_quality: options.image_quality ?? "", aspect_ratio: options.aspect_ratio ?? "", output_count: String(options.output_count ?? 1), reference_count: String(count) }).toString() : "";
  const key = query && attachments.ready && attachments.artifactIds.length === count
    ? JSON.stringify([query, ...attachments.artifactIds]) : "";
  const [request, setRequest] = useState<QuoteRequest>({ key, attempt: 0 });
  const [result, setResult] = useState<{ request: QuoteRequest; credits?: number; failed?: boolean } | null>(null);
  // Invalidate synchronously, including remove/re-add and retry of the same query.
  if (request.key !== key) setRequest({ key, attempt: request.attempt + 1 });
  useEffect(() => {
    if (!key || request.key !== key) return;
    const controller = new AbortController();
    void webBrowserFetch(`/web/v1/image-reference-quote?${query}`, { signal: controller.signal })
      .then(async (response) => {
        if (!response.ok) throw new Error();
        const payload: unknown = await response.json();
        if (!payload || typeof payload !== "object" || !("credits" in payload) || typeof payload.credits !== "number" || !Number.isSafeInteger(payload.credits) || payload.credits <= 0) throw new Error();
        if (!controller.signal.aborted) setResult({ request, credits: payload.credits });
      })
      .catch(() => { if (!controller.signal.aborted) setResult({ request, failed: true }); });
    return () => controller.abort();
  }, [query, key, request]);
  const currentResult = key && request.key === key && result?.request === request ? result : null;
  return {
    retry: () => setRequest(value => ({ ...value, attempt: value.attempt + 1 })),
    active: !!query,
    cost: currentResult?.credits,
    ready: !query || currentResult?.credits !== undefined,
    error: currentResult?.failed ? msg("useReferenceQuote.couldNotCalculateTheCostWithAttachments") : null,
  };
}
