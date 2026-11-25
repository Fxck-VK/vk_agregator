"use client";

import { useEffect, useState } from "react";
import { z } from "zod";
import { useReleaseView } from "@/features/release/ReleaseViewProvider";
import Link from "@/i18n/Link";
import { useDictionary } from "@/i18n/LocaleProvider";
import { webBrowserFetch } from "@/lib/web-api/browser";
import type { WebApiPath } from "@/lib/web-api/path";
import styles from "./ConversationHistory.module.css";

const uuid = z.string().uuid();

export function PresentationReturnLink({ conversationId, jobId }: {
  conversationId: string;
  jobId?: string;
}) {
  const { view } = useReleaseView();
  if (view !== "dev" || !uuid.safeParse(conversationId).success) return null;
  const job = uuid.safeParse(jobId);
  return <VerifiedPresentationReturnLink key={conversationId + ":" + (job.success ? job.data : "")}
    conversationId={conversationId} jobId={job.success ? job.data : undefined} />;
}

function VerifiedPresentationReturnLink({ conversationId, jobId }: {
  conversationId: string;
  jobId?: string;
}) {
  const t = useDictionary();
  const [verified, setVerified] = useState(false);

  useEffect(() => {
    const request = new AbortController();
    const target = jobId ? "?job_id=" + jobId : "";
    void webBrowserFetch(("/web/v1/presentations/" + conversationId + target) as WebApiPath, {
      method: "HEAD",
      cache: "no-store",
      signal: request.signal,
    }).then(response => {
      if (!request.signal.aborted && response.status === 204) setVerified(true);
    }).catch(() => {});
    return () => request.abort();
  }, [conversationId, jobId]);

  if (!verified) return null;
  const query = new URLSearchParams({ run: conversationId });
  if (jobId) query.set("job", jobId);
  return (
    <Link className={styles.presentationReturn} href={"/app/presentations?" + query.toString()} prefetch={false}>
      {t.conversations.returnToPresentation}
    </Link>
  );
}
