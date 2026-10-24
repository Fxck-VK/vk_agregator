"use client";

import { useEffect, useRef, useState } from "react";
import { LandingDialog } from "@/components/public/landing/LandingDialog";
import { LandingArrow, LandingToolIcon } from "@/components/public/landing/LandingPrimitives";
import { loadModelCatalog } from "@/features/models/model-catalog-cache";
import { searchModels } from "@/features/models/model-search";
import { ModelIcon } from "@/features/models/ModelIcon/ModelIcon";
import { modelIconRegistry } from "@/features/models/model-icon-registry";
import Link from "@/i18n/Link";
import type { HomeEnglishCopy } from "@/i18n/public/home-en";
import styles from "./PublicHome.module.css";

export function HomeSearch({ copy, onClose }: { copy: HomeEnglishCopy; onClose: () => void }) {
  const [query, setQuery] = useState("");
  const [models, setModels] = useState<readonly { id: string; name: string }[]>([]);
  const [status, setStatus] = useState<"loading" | "ready" | "error">("loading");
  const results = useRef<HTMLDivElement>(null);
  const input = useRef<HTMLInputElement>(null);
  useEffect(() => {
    let active = true;
    void loadModelCatalog().then(catalog => { if (active) { setModels(catalog.items.map(({ id, name }) => ({ id, name }))); setStatus("ready"); } }).catch(() => { if (active) setStatus("error"); });
    return () => { active = false; };
  }, []);
  const tools = searchModels(copy.tools.items.map(item => ({ ...item, id: item.title, name: item.title })), query);
  const found = searchModels(models, query);
  const resultLinks = () => Array.from(results.current?.querySelectorAll<HTMLAnchorElement>("a[href]") ?? []);
  return <LandingDialog title={copy.controls.search} closeLabel={copy.controls.close} onClose={onClose}>
    <div className={styles.searchField}><svg aria-hidden="true" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5"><circle cx="10" cy="10" r="6.5" /><path d="m15 15 6 6" /></svg><input ref={input} type="search" aria-label={copy.controls.searchPlaceholder} placeholder={copy.controls.searchPlaceholder} value={query} onChange={event => setQuery(event.target.value)} onKeyDown={event => { if (event.key === "ArrowDown") { event.preventDefault(); resultLinks()[0]?.focus(); } }} /></div>
    <p className={styles.searchHint}>{copy.controls.searchHint}</p>
    <div className={styles.searchResults} ref={results} onKeyDown={event => { const links = resultLinks(); const index = links.indexOf(document.activeElement as HTMLAnchorElement); if (event.key === "ArrowDown") { event.preventDefault(); links[(index + 1) % links.length]?.focus(); } else if (event.key === "ArrowUp") { event.preventDefault(); if (index <= 0) input.current?.focus(); else links[index - 1]?.focus(); } }}>
      {tools.length ? <><h3>{copy.controls.toolResults}</h3>{tools.map(tool => <Link key={tool.id} href={tool.href}><LandingToolIcon name={tool.icon} /><span>{tool.name}</span><LandingArrow /></Link>)}</> : null}
      {status === "loading" ? <p role="status">{copy.controls.loading}</p> : null}
      {status === "error" ? <p role="status">{copy.controls.modelError}</p> : null}
      {found.length ? <><h3>{copy.controls.modelResults}</h3>{found.map(model => <Link key={model.id} href={`/app?model=${encodeURIComponent(model.id)}`}><ModelIcon src={modelIconRegistry[model.id]} className={styles.searchModelIcon} /><span>{model.name}</span><LandingArrow /></Link>)}</> : null}
      {status === "ready" && !tools.length && !found.length ? <p role="status">{copy.controls.noResults}</p> : null}
    </div>
  </LandingDialog>;
}
