"use client";

import { useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/Button/Button";
import { LoadingIndicator, StateNotice } from "@/components/ui/AsyncState/AsyncState";
import { useDictionary, useLocale } from "@/i18n/LocaleProvider";
import Link from "@/i18n/Link";
import { webBrowserFetch, webBrowserMutation } from "@/lib/web-api/browser";
import { legalStatusSchema, sameLegalRevision, type LegalMetadata, type LegalStatus } from "./model";
import styles from "./Legal.module.css";

type Props = { metadata: LegalMetadata; accountId: string; preview?: boolean };
export function LegalConsentPreferences(props: Props) {
  return <Preferences key={`${props.accountId}:${props.metadata.checksum}`} {...props} />;
}
function Preferences({ metadata, accountId, preview }: Props) {
  const { legal: t } = useDictionary();
  const locale = useLocale();
  const [status, setStatus] = useState<LegalStatus | null>(null);
  const [failed, setFailed] = useState(false);
  const [pending, setPending] = useState(false);
  const [optIn, setOptIn] = useState(false);
  const [saved, setSaved] = useState(false);
  const [attempt, setAttempt] = useState(0);
  const request = useRef<{ key: string; body: string } | null>(null);
  const mutation = useRef<AbortController | null>(null);
  useEffect(() => {
    if (preview || !metadata.published) return;
    const controller = new AbortController();
    void webBrowserFetch("/web/v1/account/legal", { signal: controller.signal }).then(async response => {
      if (!response.ok) throw new Error("Legal status unavailable");
      const result = legalStatusSchema.parse(await response.json());
      if (!controller.signal.aborted) { setStatus(result); setOptIn(result.marketing_opt_in); }
    }).catch(() => { if (!controller.signal.aborted) setFailed(true); });
    return () => controller.abort();
  }, [accountId, metadata.published, preview, attempt]);
  useEffect(() => () => mutation.current?.abort(), []);
  const drift = status !== null && !sameLegalRevision(metadata, status);
  async function save(withdraw = false) {
    if (pending || preview || (!withdraw && (!metadata.published || !status || drift))) return;
    setPending(true); setFailed(false); setSaved(false);
    const body = JSON.stringify({ version: metadata.marketing_version, checksum: metadata.marketing_checksum,
      opt_in: withdraw ? false : optIn, recipient_version: status?.recipient_version ?? "", locale });
    if (request.current?.body !== body) request.current = { body, key: crypto.randomUUID() };
    const controller = new AbortController(); mutation.current = controller;
    try {
      const response = await webBrowserMutation("/web/v1/account/legal/marketing", {
        method: "POST", headers: { "Content-Type": "application/json", "X-Idempotency-Key": request.current.key }, body, signal: controller.signal,
      });
      if (!response.ok) throw new Error("Legal choice unavailable");
      const result = legalStatusSchema.parse(await response.json());
      if (!withdraw && !sameLegalRevision(metadata, result)) throw new Error("Legal documents changed");
      if (!controller.signal.aborted) { setStatus(result); setOptIn(result.marketing_opt_in); setSaved(true); request.current = null; }
    } catch { if (!controller.signal.aborted) setFailed(true); }
    finally { if (!controller.signal.aborted) setPending(false); }
  }
  const disabled = preview || !metadata.published;
  return <section className={styles.preferences} aria-labelledby="legal-preferences-title"><div className={styles.choice}>
    <h2 id="legal-preferences-title">{t.preferences}</h2>
    <div className={styles.documentLinks}>{["user-agreement", "privacy-policy", "paid-access", "payment-details"].map(slug => <Link key={slug} href={`/legal/${slug}`} target="_blank" rel="noopener noreferrer">{t.titles[slug as keyof typeof t.titles]}</Link>)}</div>
    {disabled ? <p>{preview ? t.preview : t.paused}</p> : drift ? <StateNotice inline kind="error">{t.changed}<Button variant="outline" onClick={() => window.location.reload()}>{t.reload}</Button></StateNotice> : !status && !failed ? <LoadingIndicator label={t.loading} /> : status ? <>
      <label className={styles.check}><input type="checkbox" checked={optIn} disabled={pending || !status.marketing_available || !status.recipient_version} onChange={event => { setOptIn(event.target.checked); setSaved(false); }} /><span>{t.advertising}<small>{t.optional}{" "}<Link href="/legal/marketing-consent" target="_blank" rel="noopener noreferrer">{t.titles["marketing-consent"]}</Link></small></span></label>
      {!status.marketing_available ? <p className={styles.muted}>{t.noEmail}</p> : null}
      <Button disabled={pending} onClick={() => void save()}>{pending ? t.pending : t.save}</Button>
    </> : null}
    {!preview ? <Button variant="outline" disabled={pending} onClick={() => void save(true)}>{t.withdraw}</Button> : null}
    {failed ? <StateNotice inline kind="error">{t.error}{!status && !disabled ? <Button variant="outline" onClick={() => { setFailed(false); setAttempt(value => value + 1); }}>{t.retry}</Button> : null}</StateNotice> : null}
    {saved ? <p role="status">{t.saved}</p> : null}
    <div className={styles.actions}><a href="mailto:support@epifera.com">{t.support}</a></div>
  </div></section>;
}
