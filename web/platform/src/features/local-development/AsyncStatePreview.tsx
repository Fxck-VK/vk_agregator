"use client";

import { AttachmentPreviewDialog } from "@/components/media/AttachmentPreviewDialog/AttachmentPreviewDialog";
import { CreditAmount } from "@/components/ui/CreditAmount/CreditAmount";
import { useState } from "react";
import { WorkspacePageFrame } from "@/components/layout/WorkspacePageFrame/WorkspacePageFrame";
import { LoadingIndicator, Skeleton, SkeletonGrid, StateNotice } from "@/components/ui/AsyncState/AsyncState";
import { ModeSwitchPanel } from "@/components/ui/ModeSwitchPanel/ModeSwitchPanel";
import { FileCard } from "@/features/files/FileCard/FileCard";
import { useDictionary, useMessages } from "@/i18n/LocaleProvider";
import Link from "@/i18n/Link";
import type { ImageJob, ImageJobResult } from "@/lib/web-api/contracts";
import styles from "./AsyncStatePreview.module.css";
import { ImageGenerationStatePreview } from "./ImageGenerationStatePreview";

export function AsyncStatePreview({ job, result }: { job: ImageJob; result: ImageJobResult }) {
  const t = useDictionary();
  const msg = useMessages();
  const [state, setState] = useState("loading");
  const [previewTrigger, setPreviewTrigger] = useState<HTMLButtonElement | null>(null);
  const retry = { label: t.files.retry, onClick: () => setState("ready") };
  return <WorkspacePageFrame><div className={styles.page}>
    <header><h1>{msg("localDevelopment.states")}</h1><p>{msg("localDevelopment.statesDescription")}</p></header>
    <ModeSwitchPanel activeID={state} ariaLabel={msg("localDevelopment.states")} items={[
      { id: "loading", label: msg("localDevelopment.loadingState") }, { id: "error", label: msg("localDevelopment.error") }, { id: "ready", label: msg("localDevelopment.success") },
    ]} onChange={setState} />
    <ImageGenerationStatePreview state={state} result={result} onRetry={() => setState("loading")} />
    <section id="media"><h2>{msg("localDevelopment.mediaStates")}</h2><div className={styles.card}>
      <FileCard isRetrying={false} job={job} result={state === "ready" ? result : null} resultState={state === "error" ? "error" : "loading"}
        onRequestResult={() => { if (state === "error") setState("ready"); }} onRetryJob={() => undefined}
        onOpenPreview={(_job, _artifact, trigger) => setPreviewTrigger(trigger)} />
    </div>{previewTrigger ? <AttachmentPreviewDialog items={[{ id: result.artifacts[0].id, src: `/web/v1/image-artifacts/${result.artifacts[0].id}`, alt: job.prompt }]} selectedIndex={0} onSelect={() => undefined} onClose={() => setPreviewTrigger(null)} returnFocusTo={previewTrigger} /> : null}<Link href="/app/files?category=images">{t.files.title}</Link></section>
    <section id="lists"><h2>{msg("localDevelopment.listStates")}</h2>
      {state === "loading" ? <SkeletonGrid count={3} label={t.files.loading} /> : <StateNotice kind={state === "error" ? "error" : "empty"} action={state === "error" ? retry : undefined}>{state === "error" ? t.files.loadFailure : t.conversations.mediaLibraryEmptyGenerated}</StateNotice>}
      <Link href="/app/models">{t.modelsCatalog.title}</Link>
    </section>
    <section id="actions"><h2>{msg("localDevelopment.actionStates")}</h2>
      <StateNotice kind={state === "loading" ? "loading" : state === "error" ? "error" : "success"} action={state === "error" ? retry : undefined}>
        {state === "loading" ? t.imageGeneration.statusWorking : state === "error" ? t.conversations.messageNotSent : t.imageGeneration.statusReady}
      </StateNotice><Link href="/app">{msg("paymentReturn.returnToWorkspace")}</Link>
    </section>
    <section id="notices"><h2>{msg("localDevelopment.noticeStates")}</h2>
      <StateNotice kind="error" action={retry}>{msg("useChatAttachments.uploadNetworkError")}</StateNotice>
    </section>
    <section id="data"><h2>{msg("localDevelopment.dataStates")}</h2>
      {state === "error" ? <StateNotice kind="error" action={retry}>{t.workspace.sessionRetryableError}</StateNotice> : state === "ready" ? <CreditAmount value={1000} /> : <div className={styles.data}>
        <LoadingIndicator label={t.workspace.balanceLoading} /><Skeleton className={styles.heading} /><Skeleton className={styles.input} />
      </div>}
    </section>
  </div></WorkspacePageFrame>;
}
