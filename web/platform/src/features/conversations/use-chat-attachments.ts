"use client";

import { LocalizedError, renderMessage, type MessageReference } from "@/i18n/errors";

import { useMessages } from "@/i18n/LocaleProvider";


import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { z } from "zod";
import type { ChatMediaAttachment } from "@/components/chat/ChatFilePicker/ChatFilePicker";
import type { GenerationModel } from "@/features/models/generation-model-catalog";
import { webBrowserFetch, webBrowserMutation } from "@/lib/web-api/browser";
import { WebNetworkError } from "@/lib/web-api/network-error";
import { attachmentFingerprint } from "./attachment-fingerprint";
import { acceptsAttachment, attachmentCountAllowed, attachmentPolicy } from "./attachment-policy";

const uploadSchema = z.object({ artifact_id: z.string().uuid(), mime_type: z.enum(["image/png", "image/jpeg"]), size_bytes: z.number().positive(), width: z.number().positive(), height: z.number().positive() }).strict();
type Attachment = ChatMediaAttachment & { fingerprint: string; artifactId?: string; status: "uploading" | "ready" | "failed"; progress: number | null; sizeBytes: number; error?: MessageReference };

function attachmentTotalBytes(items: readonly Pick<Attachment, "sizeBytes">[]) {
  return items.reduce((total, item) => total + item.sizeBytes, 0);
}

export function useChatAttachments(model?: GenerationModel) {
  const msg = useMessages();
  const [items, setItems] = useState<Attachment[]>([]);
  const [error, setError] = useState<MessageReference | null>(null);
  const [duplicateNotice, setDuplicateNotice] = useState(false);
  const [networkNotice, setNetworkNotice] = useState(false);
  const [checking, setChecking] = useState(false);
  const current = useRef<Attachment[]>([]);
  const pending = useRef(new Map<string, AbortController>());
  const selections = useRef(new Set<AbortController>());
  const selectionQueue = useRef(Promise.resolve());
  const mounted = useRef(true);
  const previews = useRef(new Set<string>());
  const policy = useMemo(() => attachmentPolicy(model), [model]);
  const { maxCount, maxBytes, maxTotalBytes } = policy;
  const validationContext = `${model?.id ?? ""}:${JSON.stringify(policy)}`;
  const activeContext = useRef({ key: validationContext });
  if (activeContext.current.key !== validationContext) activeContext.current = { key: validationContext };
  const selectionContext = activeContext.current;
  const [errorContext, setErrorContext] = useState(validationContext);
  // Notices describe an attempted selection for one model. Existing files are
  // retained and checked against the new model separately below.
  if (errorContext !== validationContext) {
    setErrorContext(validationContext);
    setError(null);
  }
  const update = useCallback((next: Attachment[]) => { current.current = next; setItems(next); }, []);

  useEffect(() => {
    mounted.current = true;
    const requests = pending.current;
    const checks = selections.current;
    const urls = previews.current;
    return () => { mounted.current = false; checks.forEach(check => check.abort()); checks.clear(); requests.forEach((request) => request.abort()); requests.clear(); urls.forEach((url) => URL.revokeObjectURL(url)); urls.clear(); };
  }, []);

  useEffect(() => {
    const checks = selections.current;
    return () => {
      checks.forEach(check => check.abort());
      checks.clear();
      selectionQueue.current = Promise.resolve();
    };
  }, [validationContext]);

  const upload = useCallback(async (item: Attachment, modelID: string, byteLimit: number) => {
    const controller = new AbortController();
    pending.current.set(item.id, controller);
    try {
      const file = item.file;
      if (!file || file.size === 0 || file.size > byteLimit) throw new LocalizedError("useChatAttachments.fileSizeLimit", { value1: byteLimit / (1024 * 1024) });
      const body = new FormData(); body.set("file", file);
      const response = await webBrowserMutation(`/web/v1/input-artifacts?model_id=${encodeURIComponent(modelID)}`, {
        method: "POST", body, signal: controller.signal,
        onUploadProgress: (progress) => {
          if (!controller.signal.aborted) update(current.current.map((value) => value.id === item.id ? { ...value, progress } : value));
        },
      });
      if (!response.ok) throw new LocalizedError(response.status === 413 ? "useChatAttachments.theFileIsTooLargeMaximumSize" : response.status === 400 ? "useChatAttachments.couldNotAcceptTheImageUsePng" : response.status === 429 ? "useChatAttachments.tooManyUploadsPleaseTryAgainLater" : "useChatAttachments.couldNotUploadTheFilePleaseTry");
      const uploaded = uploadSchema.parse(await response.json());
      if (!controller.signal.aborted) {
        if (current.current.some((value) => value.id !== item.id && value.artifactId === uploaded.artifact_id)) {
          if (item.previewUrl && previews.current.delete(item.previewUrl)) URL.revokeObjectURL(item.previewUrl);
          update(current.current.filter((value) => value.id !== item.id));
          setDuplicateNotice(true);
        } else update(current.current.map((value) => value.id === item.id ? { ...value, mimeType: uploaded.mime_type, artifactId: uploaded.artifact_id, status: "ready", progress: 100, sizeBytes: uploaded.size_bytes, error: undefined } : value));
      }
    } catch (reason) {
      if (!controller.signal.aborted) {
        const networkFailure = reason instanceof WebNetworkError;
        if (networkFailure) setNetworkNotice(true);
        update(current.current.map((value) => value.id === item.id ? { ...value, status: "failed", error: networkFailure
          ? { key: "useChatAttachments.uploadNetworkError" }
          : reason instanceof LocalizedError ? { key: reason.key, parameters: reason.parameters }
          : { key: "useChatAttachments.couldNotUploadTheFilePleaseTry" } } : value));
      }
    } finally { if (pending.current.get(item.id) === controller) pending.current.delete(item.id); }
  }, [update]);

  const add = useCallback((selected: ChatMediaAttachment[]) => {
    if (!selected.length) return;
    if (!maxCount || !model) { setError({ key: "useChatAttachments.theSelectedModelDoesNotSupportAttachments" }); return; }
    if (selected.some((item) => !acceptsAttachment(policy, item))) { setError({ key: "useChatAttachments.acceptedFormats", parameters: { value1: [...new Set(policy.formats.map(format => format.mime === "image/png" ? "PNG" : "JPEG"))].join(msg("useChatAttachments.formatSeparator")) } }); return; }
    if (selected.some((item) => item.file && (item.file.size === 0 || item.file.size > maxBytes))) { setError({ key: "useChatAttachments.fileSizeLimit", parameters: { value1: maxBytes / (1024 * 1024) } }); return; }
    setError(null);
    const controller = new AbortController();
    selections.current.add(controller);
    setChecking(true);
    // Serialize selections so drop/paste/picker events cannot race the same bytes.
    const task = selectionQueue.current.then(async () => {
      controller.signal.throwIfAborted();
      if (activeContext.current !== selectionContext) return;
      setError(null);
      const unique: Attachment[] = [];
      let duplicate = false;
      for (const item of selected) {
        controller.signal.throwIfAborted();
        if ([...current.current, ...unique].some(value => value.id === item.id)) {
          duplicate = true;
          continue;
        }
        let file = item.file;
        if (!file && item.source === "generated" && z.string().uuid().safeParse(item.id).success) {
          const response = await webBrowserFetch(`/web/v1/image-artifacts/${item.id}`, { signal: controller.signal });
          if (!response.ok) throw new LocalizedError("useChatAttachments.couldNotOpenTheImage");
          const blob = await response.blob();
          file = new File([blob], item.name, { type: blob.type });
        }
        if (activeContext.current !== selectionContext) return;
        if (!file || file.size === 0 || file.size > maxBytes) throw new LocalizedError("useChatAttachments.fileSizeLimit", { value1: maxBytes / (1024 * 1024) });
        if (!acceptsAttachment(policy, { ...item, file, mimeType: file.type })) throw new LocalizedError("useChatAttachments.couldNotAcceptTheImageUsePng");
        const fingerprint = await attachmentFingerprint(file, controller.signal);
        if ([...current.current, ...unique].some(value => value.fingerprint === fingerprint)) {
          duplicate = true;
          continue;
        }
        unique.push({ ...item, file, fingerprint, status: "uploading", progress: null, sizeBytes: file.size });
      }
      controller.signal.throwIfAborted();
      if (activeContext.current !== selectionContext) return;
      if (duplicate) setDuplicateNotice(true);
      if (current.current.length + unique.length > maxCount) {
        setError({ key: "useChatAttachments.youCanAttachUpToValueImages", parameters: { value1: maxCount } });
        return;
      }
      if (attachmentTotalBytes([...current.current, ...unique]) > maxTotalBytes) {
        setError({ key: "useChatAttachments.totalAttachmentsTooLargeForThisModel" });
        return;
      }
      const additions = unique.map(item => {
        const previewUrl = item.file && typeof URL.createObjectURL === "function" ? URL.createObjectURL(item.file) : item.previewUrl;
        if (item.file && previewUrl) previews.current.add(previewUrl);
        return { ...item, previewUrl };
      });
      update([...current.current, ...additions]);
      for (const item of additions) void upload(item, model.id, maxBytes);
    }).catch(reason => {
      if (!controller.signal.aborted && activeContext.current === selectionContext) setError(reason instanceof LocalizedError
        ? { key: reason.key, parameters: reason.parameters }
        : { key: "useChatAttachments.couldNotCheckFile" });
    }).finally(() => {
      selections.current.delete(controller);
      if (mounted.current) setChecking(selections.current.size > 0);
    });
    selectionQueue.current = task;
    return task;
  }, [maxCount, maxBytes, maxTotalBytes, model, msg, policy, selectionContext, update, upload]);

  const remove = (id: string) => {
    pending.current.get(id)?.abort(); pending.current.delete(id);
    const item = current.current.find((value) => value.id === id);
    if (item?.previewUrl && previews.current.delete(item.previewUrl)) URL.revokeObjectURL(item.previewUrl);
    update(current.current.filter((value) => value.id !== id)); setError(null); setDuplicateNotice(false);
  };
  const clear = () => {
    selections.current.forEach(controller => controller.abort());
    selections.current.clear();
    setChecking(false);
    setNetworkNotice(false);
    setDuplicateNotice(false);
    setError(null);
    for (const item of current.current) remove(item.id);
  };
  const retry = (id: string) => {
    const item = current.current.find((value) => value.id === id);
    if (!item || !model || !maxCount || item.status !== "failed") return;
    if (!acceptsAttachment(policy, item) || item.sizeBytes > maxBytes) return;
    update(current.current.map((value) => value.id === id ? { ...value, status: "uploading", progress: null, error: undefined } : value));
    void upload(item, model.id, maxBytes);
  };
  const totalBytesError = attachmentTotalBytes(items) > maxTotalBytes ? msg("useChatAttachments.totalAttachmentsTooLargeForThisModel") : null;
  const incompatible = items.length > maxCount || items.some(item => !acceptsAttachment(policy, item) || item.sizeBytes > maxBytes);
  const countError = !attachmentCountAllowed(policy, items.length) ? msg("useChatAttachments.allowedImageCounts", { value1: policy.allowedCounts?.filter(count => count > 0).join(", ") ?? `1–${maxCount}` }) : null;
  const modelError = incompatible ? msg("useChatAttachments.theAttachmentsAreNotCompatibleWithThe") : totalBytesError ?? countError;
  return { enabled: policy.enabled, accept: policy.accept, maxCount, items: items.map(item => ({
    id: item.id,
    mimeType: item.mimeType,
    name: item.name,
    source: item.source,
    ...(item.file ? { file: item.file } : {}),
    ...(item.previewUrl ? { previewUrl: item.previewUrl } : {}),
    fingerprint: item.fingerprint,
    ...(item.artifactId ? { artifactId: item.artifactId } : {}),
    status: item.status,
    progress: item.progress,
    error: renderMessage(msg, item.error) ?? undefined,
  })), add, remove, retry, clear, duplicateNotice, dismissDuplicateNotice: () => setDuplicateNotice(false),
    networkNotice: networkNotice && items.some(item => item.error?.key === "useChatAttachments.uploadNetworkError"), dismissNetworkNotice: () => setNetworkNotice(false),
    error: modelError ?? renderMessage(msg, error), blocked: checking || !!modelError || items.some((item) => item.status !== "ready"), ids: items.flatMap((item) => item.artifactId ? [item.artifactId] : []) };
}

export type ChatAttachments = ReturnType<typeof useChatAttachments>;
