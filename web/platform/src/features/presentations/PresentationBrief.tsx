"use client";

import { useRef, useState, useSyncExternalStore, type FormEvent } from "react";
import { Button } from "@/components/ui/Button/Button";
import { InputSurface } from "@/components/ui/InputSurface/InputSurface";
import { presentationBriefRu } from "@/i18n/presentation-brief";
import { PresentationSelect } from "./PresentationSelect";
import styles from "./PresentationsPage.module.css";
import prices from "@/features/session/presentation-prices.preview.json";

const subscribe = () => () => {};
const browserSnapshot = () => true;
const serverSnapshot = () => false;

const copy = presentationBriefRu;
const counts = copy.options.counts;
const stylesOfDeck = copy.options.styles;
const covers = copy.options.covers;

function starLabel(price: number) {
  const form = new Intl.PluralRules(copy.locale).select(price);
  return `${price} ${form === "one" ? copy.stars.one : form === "few" ? copy.stars.few : copy.stars.other}`;
}

export function PresentationBrief() {
  const ready = useSyncExternalStore(subscribe, browserSnapshot, serverSnapshot);
  const [brief, setBrief] = useState("");
  const [status, setStatus] = useState("");
  const [count, setCount] = useState<(typeof counts)[number]["value"]>(copy.defaults.count);
  const [deckStyle, setDeckStyle] = useState<(typeof stylesOfDeck)[number]["value"]>(copy.defaults.style);
  const [cover, setCover] = useState<(typeof covers)[number]["value"]>(copy.defaults.cover);
  const output = useRef<HTMLTextAreaElement>(null);

  function clearBrief() {
    setBrief("");
    setStatus("");
  }

  function assemble(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    const read = (field: string) => String(data.get(field) ?? "").trim();
    const topic = read("topic");
    if (!topic) {
      setStatus(copy.status.missingTopic);
      event.currentTarget.querySelector("input")?.focus();
      return;
    }

    const audience = read("audience");
    const notes = read("notes");
    const lines = [copy.output.title, "", `${copy.output.topic}: ${topic}`];
    if (audience) lines.push(`${copy.output.audience}: ${audience}`);
    lines.push(`${copy.output.slides}: ${count}`, `${copy.output.style}: ${deckStyle}`, `${copy.output.illustrations}: ${covers.find(option => option.value === cover)?.label}`, copy.output.formats);
    if (notes) lines.push("", copy.output.requirements, notes);
    setBrief(lines.join("\n"));
    setStatus(copy.status.ready);
  }

  async function copyBrief() {
    try {
      await navigator.clipboard.writeText(brief);
      if (output.current?.value === brief) setStatus(copy.status.copied);
    } catch {
      const field = output.current;
      if (field?.value !== brief) return;
      field.focus();
      field.select();
      setStatus(copy.status.selected);
    }
  }

  function downloadBrief() {
    const url = URL.createObjectURL(new Blob([brief], { type: "text/plain;charset=utf-8" }));
    const link = document.createElement("a");
    link.href = url;
    link.download = "epifera-presentation-brief.txt";
    link.click();
    window.setTimeout(() => URL.revokeObjectURL(url), 1000);
    setStatus(copy.status.downloaded);
  }

  return <section className={styles.brief} aria-labelledby="presentation-brief-heading">
    <header><h2 id="presentation-brief-heading">{copy.heading}</h2><p>{copy.description}</p></header>
    <form method="post" className={styles.form} onSubmit={assemble} onChange={clearBrief}>
      <div className={styles.field}><label htmlFor="presentation-topic">{copy.form.topic}</label>
        <InputSurface className={styles.input}><input id="presentation-topic" name="topic" placeholder={copy.form.topicPlaceholder} maxLength={250} required /></InputSurface>
      </div>
      <div className={styles.field}><label htmlFor="presentation-audience">{copy.form.audience} <small>{copy.form.optional}</small></label>
        <InputSurface className={styles.input}><input id="presentation-audience" name="audience" placeholder={copy.form.audiencePlaceholder} maxLength={160} /></InputSurface>
      </div>
      <div className={styles.row}>
        <PresentationSelect label={copy.form.slideCount} value={count} options={counts} onChange={value => { setCount(value); clearBrief(); }} />
        <PresentationSelect label={copy.form.style} value={deckStyle} options={stylesOfDeck} onChange={value => { setDeckStyle(value); clearBrief(); }} />
      </div>
      <div className={styles.field}><label htmlFor="presentation-notes">{copy.form.notes} <small>{copy.form.optional}</small></label>
        <InputSurface className={styles.input}><textarea id="presentation-notes" name="notes" rows={3} maxLength={5000} placeholder={copy.form.notesPlaceholder} /></InputSurface>
      </div>
      <PresentationSelect label={copy.form.illustration} value={cover} options={covers} onChange={value => { setCover(value); clearBrief(); }} />
      <div className={styles.price} aria-live="polite"><span>{copy.form.estimate}</span><strong>{starLabel(prices[cover])}</strong><small>{copy.form.freeExample}</small></div>
      <Button type="submit" className={styles.submit} disabled={!ready}>{copy.form.submit}</Button>
    </form>
    {brief && <div className={styles.result}>
      <div className={styles.field}><label htmlFor="presentation-output">{copy.form.result}</label>
        <InputSurface className={styles.input}><textarea id="presentation-output" ref={output} value={brief} readOnly rows={7} /></InputSurface>
      </div>
      <div className={styles.actions}>
        <Button variant="outline" onClick={copyBrief}>{copy.form.copy}</Button>
        <Button variant="outline" onClick={downloadBrief}>{copy.form.download}</Button>
      </div>
    </div>}
    <p className={styles.status} role="status">{status}</p>
  </section>;
}
