"use client";

import { useCallback, useEffect, useRef, useState, type FormEvent } from "react";
import { InputSurface } from "@/components/ui/InputSurface/InputSurface";
import { Button } from "@/components/ui/Button/Button";
import { LandingArrow } from "@/components/public/landing/LandingPrimitives";
import { HomePicture } from "@/features/landing-variants/third/HomeMedia";
import { presentationStyles } from "@/i18n/presentation-deck";
import { presentationPageCopy, presentationStudioCopy as labels } from "@/i18n/presentation-studio";
import { useRouter, useSearchParams } from "@/i18n/navigation";
import Link from "@/i18n/Link";
import { webBrowserFetch } from "@/lib/web-api/browser";
import { parseAccountProfile } from "@/lib/web-api/contracts";
import { PresentationExample } from "./PresentationExample";
import { DeckRequest } from "./DeckRequest";
import { DeckResult } from "./DeckResult";
import { defaultDeckAppearance, type DeckAppearance } from "./deck-appearance";
import { downloadFile, emptyBrief, presentationPrompt, readDeck, type Brief, type DeckSlide } from "./deck-content";
import { getDeckAppearance, getDeckDraft, parseDeckRun, readDeckBrief, replaceDeckRun, saveDeckAppearance, saveDeckDraft, type DeckRun } from "./deck-run";
import styles from "./Studio.module.css";

const scenarios = Object.values(presentationStyles);
// In-memory handoff across the server-verified guest → account remount in this tab.
let loginDraft: { accountId: string; brief: Brief } | null = null;

export function PresentationStudio({ accountId, local, localApiUnavailable = false, initialRun }: {
  accountId?: string;
  local: boolean;
  localApiUnavailable?: boolean;
  initialRun?: DeckRun;
}) {
  const router = useRouter();
  const searchParams = useSearchParams();
  const [brief, setBrief] = useState<Brief>(() => loginDraft && loginDraft.accountId === accountId ? loginDraft.brief : emptyBrief);
  // Browser Back can restore the cached form props after the job URL was set through history.replaceState.
  const [run, setRun] = useState(() => initialRun ?? (accountId && searchParams ? parseDeckRun(searchParams) : undefined));
  const [step, setStep] = useState(() => run || loginDraft && loginDraft.accountId === accountId ? 1 : 0);
  const [loginOpened, setLoginOpened] = useState(false);
  useEffect(() => { if (accountId === loginDraft?.accountId) loginDraft = null; }, [accountId]);
  const [mode, setMode] = useState<"external" | "account">(!run && (localApiUnavailable || local && !accountId) ? "external" : "account");
  const [source, setSource] = useState("");
  const [slides, setSlides] = useState<DeckSlide[]>([]);
  const [appearance, setAppearance] = useState(defaultDeckAppearance);
  const [busy, setBusy] = useState(false);
  const [editing, setEditing] = useState(false);
  const [notice, setNotice] = useState("");
  const [problem, setProblem] = useState("");
  const [copyFallback, setCopyFallback] = useState(false);
  const heading = useRef<HTMLHeadingElement>(null);
  const promptField = useRef<HTMLTextAreaElement>(null);
  const previousStep = useRef(step);
  const prompt = presentationPrompt(brief);
  useEffect(() => {
    if (previousStep.current !== step) heading.current?.focus();
    previousStep.current = step;
  }, [step]);

  function update<K extends keyof Brief>(key: K, value: Brief[K]) {
    setBrief(current => ({ ...current, [key]: value }));
    setProblem("");
  }
  function advance(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!brief.topic.trim()) { setProblem(labels.topicRequired); event.currentTarget.querySelector("textarea")?.focus(); return; }
    setStep(1); setNotice(""); setProblem("");
  }
  const rememberRun = useCallback((next: DeckRun) => {
    setRun(next);
    replaceDeckRun(next);
  }, []);

  const restoreBrief = useCallback((original: string) => {
    const restored = readDeckBrief(original);
    if (restored) setBrief(restored);
  }, []);

  const changeSlides = useCallback((next: DeckSlide[]) => {
    setSlides(next);
    if (accountId && run?.job) return saveDeckDraft(accountId, run.job, next);
  }, [accountId, run]);

  function changeAppearance(next: DeckAppearance) {
    setAppearance(next);
    if (accountId && run?.job) return saveDeckAppearance(accountId, run.job, next);
  }

  const receive = useCallback((input: string, original?: string, resultJob?: string) => {
    setSource(input);
    setBusy(false);
    setNotice("");
    const recovered = original ? readDeckBrief(original) : undefined;
    const count = recovered?.count ?? brief.count;
    if (recovered) setBrief(recovered);
    if (run && resultJob && run.job !== resultJob) rememberRun({ conversation: run.conversation, job: resultJob });
    const job = resultJob ?? run?.job;
    try {
      const draft = original !== undefined && accountId && job ? getDeckDraft(accountId, job) : undefined;
      const next = draft ?? readDeck(input, count);
      if (original === undefined && accountId && job) saveDeckDraft(accountId, job, next);
      setSlides(next);
      setAppearance(accountId && job ? getDeckAppearance(accountId, job) ?? defaultDeckAppearance : defaultDeckAppearance);
      setStep(2);
      setEditing(false);
      setProblem("");
    } catch (error) {
      setEditing(true);
      setProblem(error instanceof Error ? error.message : labels.outlineInvalid);
    }
  }, [accountId, brief.count, rememberRun, run]);
  async function copy() {
    try { await navigator.clipboard.writeText(prompt); setNotice(labels.promptCopied); }
    catch { setCopyFallback(true); setNotice(labels.copyPromptManually); }
  }
  async function returnFromLogin() {
    try {
      const response = await webBrowserFetch("/web/v1/me", { cache: "no-store" });
      if (!response.ok) throw new Error();
      const profile = parseAccountProfile(await response.json());
      loginDraft = { accountId: profile.account_id, brief };
      router.refresh();
    } catch { setNotice(labels.signInPending); }
  }
  useEffect(() => { if (copyFallback) { promptField.current?.focus(); promptField.current?.select(); } }, [copyFallback]);
  function backToBrief() {
    if (run) replaceDeckRun();
    setRun(undefined);
    setBusy(false);
    setSource("");
    setStep(0);
    setNotice("");
    setProblem("");
    setEditing(false);
    setCopyFallback(false);
  }
  function startOver() {
    backToBrief();
    setBrief(emptyBrief);
    setSlides([]);
  }

  return <section className={styles.studio} aria-label={labels.label}>
    <ol className={styles.steps} aria-label={labels.stepsLabel}>{labels.steps.map((label, i) =>
      <li key={label} aria-current={i === step ? "step" : undefined} data-done={i < step}><span aria-hidden="true">{i < step ? "✓" : i + 1}</span>{label}</li>)}</ol>
    {step === 2 ? <div ref={heading} tabIndex={-1} role="group" aria-label={labels.resultLabel}><DeckResult slides={slides} brief={brief} appearance={appearance} onAppearanceChange={changeAppearance} canGenerateImage={Boolean(accountId)} onChange={changeSlides} onNew={startOver} /></div> :
      <div className={styles.workbench}>
        <div className={styles.composer}>
          <h2 tabIndex={-1} ref={heading}>{step === 0 ? labels.topicHeading : editing ? labels.editingHeading : labels.prepareHeading}</h2>
          {step === 0 ? <form onSubmit={advance} className={styles.form}>
            <label htmlFor="deck-topic" className={styles.sr}>{labels.topicLabel}</label>
            <InputSurface className={styles.topic}><textarea id="deck-topic" rows={3} value={brief.topic} required maxLength={240} placeholder={labels.topicPlaceholder} onChange={e => update("topic", e.target.value)} /></InputSurface>
            <div className={styles.options}>
              <fieldset><legend>{labels.countLabel}</legend><div className={styles.choices}>{([5, 10] as const).map(n =>
                <label key={n}><input type="radio" name="count" value={n} checked={brief.count === n} onChange={() => update("count", n)} /><span>{n}</span></label>)}</div></fieldset>
              <fieldset><legend>{labels.styleLabel}</legend><div className={styles.choices}>{scenarios.map(style =>
                <label key={style}><input type="radio" name="style" checked={brief.style === style} onChange={() => update("style", style)} /><span>{style}</span></label>)}</div></fieldset>
            </div>
            <details className={styles.details}><summary>{labels.audienceAndNotes}</summary><div className={styles.more}>
              <label>{labels.audienceLabel}<input maxLength={160} value={brief.audience} onChange={e => update("audience", e.target.value)} placeholder={labels.audiencePlaceholder} /></label>
              <label>{labels.notesLabel}<textarea rows={3} maxLength={1800} value={brief.notes} onChange={e => update("notes", e.target.value)} placeholder={labels.notesPlaceholder} /></label>
            </div></details>
            {problem && <p className={styles.error} role="alert">{problem}</p>}
            <Button className={styles.primary} type="submit">{labels.continue} <LandingArrow /></Button>
          </form> : <>
            {!busy && (!run || editing) && <button className={styles.back} onClick={backToBrief}>{labels.back}</button>}
            {!editing && !run && (local || accountId) && <div className={styles.methods} role="group" aria-label={labels.generationMode}>
              <button disabled={busy} aria-pressed={mode === "account"} onClick={() => { setMode("account"); setNotice(""); }}>{labels.inEpifera}</button>
              <button disabled={busy} aria-pressed={mode === "external"} onClick={() => { setMode("external"); setNotice(""); }}>{labels.inAnotherModel}</button>
            </div>}
            {mode === "account" && !editing ? (accountId ? <DeckRequest accountId={accountId} prompt={prompt} run={run} onRun={rememberRun}
              onPrompt={restoreBrief} onResult={receive} onBusy={setBusy} onRestart={backToBrief} /> :
              <div className={styles.request}>{localApiUnavailable
                ? <><p>{labels.serverUnavailable}</p><Button variant="outline" onClick={() => setMode("external")}>{labels.useAnotherModel}</Button></>
                : <><p>{labels.signInHelp}</p><Link className={styles.primary} href="/login" target="_blank" rel="noopener" onClick={() => setLoginOpened(true)}>{presentationPageCopy.signIn}</Link>
                  {loginOpened && <Button variant="outline" onClick={returnFromLogin}>{labels.signInCompleted}</Button>}</>}</div>) : <div className={styles.form}>
              {!editing && <><p className={styles.help}>{labels.externalHelp}</p>
                <div className={styles.actions}><Button variant="outline" onClick={copy}>{labels.copyPrompt}</Button>
                  <button className={styles.textButton} onClick={() => downloadFile(new Blob([prompt], { type: "text/plain;charset=utf-8" }), "epifera-brief.txt")}>{labels.downloadBrief}</button></div>
                {copyFallback && <textarea ref={promptField} className={styles.raw} readOnly aria-label={labels.promptLabel} value={prompt} />}</>}
              <label className={styles.editor}>{editing ? labels.editOutline : labels.modelResponse}<textarea className={styles.raw} rows={editing ? 10 : 5} value={source} maxLength={32000} placeholder={labels.outlinePlaceholder} onChange={e => { setSource(e.target.value); setProblem(""); }} /></label>
              {problem && <p className={styles.error} role="alert">{problem}</p>}
              <Button className={styles.primary} disabled={!source.trim()} onClick={() => receive(source)}>{editing ? labels.saveSlides : labels.buildSlides} <LandingArrow /></Button>
            </div>}
            {notice && <p className={styles.feedback} role="status">{notice}</p>}
          </>}
        </div>
        <aside className={styles.preview} aria-label={labels.previewLabel}>
          <div className={styles.previewHead}><span>{brief.topic.trim() ? labels.coverSketch : labels.exampleDesign}</span><span>16:9</span></div>
          <div className={styles.canvas}>
            <div className={styles.cover} data-style={brief.style}>
              <div className={styles.photo}><HomePicture photo="workflowStart" original sizes="(max-width: 767px) 36vw, 215px" /></div>
              <div className={styles.coverText}><span>{brief.style === presentationStyles.educational ? labels.educationalDeck : labels.businessDeck}</span><strong>{brief.topic.trim() || labels.exampleTopic}</strong><i aria-hidden="true" /></div>
            </div>
          </div>
          <div className={styles.previewFoot}><span>{labels.slideCount(brief.count)}</span>{(accountId || local) && <PresentationExample />}</div>
        </aside>
      </div>}
  </section>;
}
