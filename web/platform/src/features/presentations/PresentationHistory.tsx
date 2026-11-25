"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { z } from "zod";
import { Button } from "@/components/ui/Button/Button";
import { MonitorIcon } from "@/components/icons/MonitorIcon";
import { presentationDeckCopy } from "@/i18n/presentation-deck";
import { presentationHistoryCopy as copy } from "@/i18n/presentation-studio";
import Link from "@/i18n/Link";
import { useLocale } from "@/i18n/LocaleProvider";
import { startGenerationPolling } from "@/lib/generation-polling";
import { webBrowserFetch } from "@/lib/web-api/browser";
import { webChatJobSchema } from "@/lib/web-api/contracts";
import { failedPresentationStatuses, presentationStages } from "./deck-generation";
import { presentationWorkspacePath } from "./route";
import styles from "./PresentationHistory.module.css";

const historySchema = z.object({
  items: z.array(webChatJobSchema.extend({
    conversation_id: z.uuid(),
    title: z.string().min(1),
    created_at: z.iso.datetime({ offset: true }),
    model_name: z.string().optional(),
    failure_code: z.string().optional(),
  })),
  next_cursor: z.string().min(1).optional(),
});

type HistoryPage = z.infer<typeof historySchema>;
type HistoryItem = HistoryPage["items"][number];

const phases = presentationDeckCopy.request;
const stageLabels = [phases.stageQueued, phases.stageWriting, phases.stagePreparing];

function getStatus(item: HistoryItem) {
  if (item.status === "succeeded") {
    return item.failure_code === "answer_missing"
      ? { tone: "failed", label: copy.missing }
      : { tone: "ready", label: copy.ready };
  }
  if (item.status === "cancelled") return { tone: "failed", label: copy.cancelled };
  if (failedPresentationStatuses.has(item.status)) return { tone: "failed", label: copy.failed };
  return { tone: "active", label: stageLabels[presentationStages(item.status)] };
}

export function PresentationHistory() {
  const locale = useLocale();
  const dateFormat = useMemo(() => new Intl.DateTimeFormat(locale, {
    day: "numeric", month: "long", year: "numeric", hour: "2-digit", minute: "2-digit",
  }), [locale]);
  const [cursors, setCursors] = useState<string[]>([]);
  const cursor = cursors.at(-1);
  const [page, setPage] = useState<HistoryPage>();
  const [failed, setFailed] = useState(false);
  const polling = useRef<ReturnType<typeof startGenerationPolling> | null>(null);

  useEffect(() => {
    let active = false;
    const tracker = startGenerationPolling({
      delayMs: 10_000,
      pauseWhenHidden: true,
      poll: async signal => {
        try {
          const query = cursor ? "?" + new URLSearchParams({ cursor }) : "";
          const response = await webBrowserFetch(`/web/v1/presentations${query}`, { signal, cache: "no-store" });
          if (!response.ok) throw new Error("presentation_history");
          const result = historySchema.parse(await response.json());
          if (signal.aborted) return false;
          active = result.items.some(item => getStatus(item).tone === "active");
          setPage(result);
          setFailed(false);
        } catch {
          if (!signal.aborted) setFailed(true);
        }
        return active;
      },
    });
    polling.current = tracker;
    return () => { tracker.stop(); polling.current = null; };
  }, [cursor]);

  function navigate(next: string[]) {
    setPage(undefined);
    setFailed(false);
    setCursors(next);
  }

  return <section className={styles.history} aria-label={copy.title}>
    {failed && <p className={styles.feedback} role="alert">{page ? copy.refreshError : copy.loadError}</p>}
    {!page && !failed && <p className={styles.feedback} role="status">{copy.loading}</p>}
    {page?.items.length === 0 && <p className={styles.feedback}>{copy.empty}</p>}
    {page && page.items.length > 0 && <ul className={styles.list}>
      {page.items.map(item => {
        const { tone, label } = getStatus(item);
        const query = new URLSearchParams({ run: item.conversation_id, job: item.job_id });
        return <li key={item.job_id}>
          <Link href={presentationWorkspacePath + "?" + query} className={styles.row} prefetch={false}>
            <span className={styles.icon} aria-hidden="true"><MonitorIcon /></span>
            <span className={styles.details}>
              <strong>{item.title}</strong>
              <span><time dateTime={item.created_at}>{dateFormat.format(new Date(item.created_at))}</time>
                {item.model_name && <> · {item.model_name}</>}</span>
            </span>
            <span className={styles.status} data-tone={tone}>{label}</span>
            <span className={styles.chevron} aria-hidden="true">›</span>
          </Link>
        </li>;
      })}
    </ul>}
    {(page || failed || cursor) && <div className={styles.actions}>
      <Button variant="outline" onClick={() => polling.current?.refresh()}>{copy.refresh}</Button>
      {(cursor || page?.next_cursor) && <div className={styles.pagination}>
        <Button variant="outline" disabled={!cursor} onClick={() => navigate(cursors.slice(0, -1))}>{copy.newer}</Button>
        <Button variant="outline" disabled={!page?.next_cursor} onClick={() => {
          if (page?.next_cursor) navigate([...cursors, page.next_cursor]);
        }}>{copy.older}</Button>
      </div>}
    </div>}
  </section>;
}
