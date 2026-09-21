import type { ComponentPropsWithoutRef, ReactNode } from "react";
import { MasonryGrid } from "@/components/ui/MasonryGrid/MasonryGrid";
import { RetryAction, type RetryActionProps } from "./RetryAction";
import styles from "./AsyncState.module.css";

export function LoadingIndicator({ label, progress, processingLabel, className }: {
  label: string; progress?: number | null; processingLabel?: string; className?: string;
}) {
  const value = progress == null ? undefined : Math.min(100, Math.max(0, progress));
  return <span className={[styles.indicator, className].filter(Boolean).join(" ")} role="progressbar"
    aria-label={label} aria-valuemin={0} aria-valuemax={100} aria-valuenow={value}
    aria-valuetext={value === 100 ? processingLabel : undefined}
    data-indeterminate={value === undefined || undefined} data-processing={value === 100 || undefined}>
    <svg aria-hidden="true" viewBox="0 0 40 40">
      <circle className={styles.track} cx="20" cy="20" r="16" />
      <circle className={styles.fill} cx="20" cy="20" r="16" pathLength="100"
        strokeDasharray={value === undefined ? "25 75" : "100"} strokeDashoffset={value === undefined ? 0 : 100 - value} />
    </svg>
  </span>;
}

export function Skeleton({ className, ...props }: ComponentPropsWithoutRef<"span">) {
  return <span {...props} aria-hidden="true" data-ui="skeleton" className={[styles.skeleton, className].filter(Boolean).join(" ")} />;
}

export function StateNotice({ kind = "empty", children, action, inline = false, className, ...props }: Omit<ComponentPropsWithoutRef<"div">, "children"> & {
  kind?: "loading" | "error" | "empty" | "success" | "info";
  children: ReactNode; action?: RetryActionProps; inline?: boolean;
}) {
  return <div {...props} className={[styles.notice, inline && styles.inline, className].filter(Boolean).join(" ")}
    data-ui="state-notice" data-kind={kind} role={props.role ?? (kind === "error" ? "alert" : "status")}>
    {kind === "loading" ? <span aria-hidden="true"><LoadingIndicator label="" /></span> : null}
    {kind === "error" ? <svg aria-hidden="true" className={styles.errorIcon} viewBox="0 0 24 24" fill="none"><circle cx="12" cy="12" r="9" stroke="currentColor" strokeWidth="1.7" /><path d="M12 7v6m0 3v1" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" /></svg> : null}
    <div className={styles.message}>{children}</div>
    {action ? <RetryAction {...action} /> : null}
  </div>;
}

export function MediaState({ state = "loading", label, retry, className, compact = false }: {
  state?: "loading" | "error"; label: string; retry?: RetryActionProps; className?: string; compact?: boolean;
}) {
  return <span className={[styles.media, className].filter(Boolean).join(" ")} data-ui="media-state" data-state={state} data-compact={compact || undefined}>
    {state === "loading" ? <LoadingIndicator label={label} /> : <>
      <span className={styles.srOnly} role="alert">{label}</span>
      {retry ? <RetryAction {...retry} iconOnly /> : <svg aria-hidden="true" className={styles.errorIcon} viewBox="0 0 24 24" fill="none"><circle cx="12" cy="12" r="9" stroke="currentColor" strokeWidth="1.7" /><path d="M12 7v6m0 3v1" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" /></svg>}
    </>}
  </span>;
}

export function SkeletonGrid({ label, count = 6, className }: { label: string; count?: number; className?: string }) {
  return <div role="status" aria-label={label} data-ui="skeleton-grid">
    <MasonryGrid aria-hidden="true" className={className}>
      {Array.from({ length: count }, (_, index) => <li key={index}><Skeleton className={styles.cardSkeleton} style={{ aspectRatio: ["3 / 4", "1", "4 / 3"][index % 3] }} /></li>)}
    </MasonryGrid>
  </div>;
}
