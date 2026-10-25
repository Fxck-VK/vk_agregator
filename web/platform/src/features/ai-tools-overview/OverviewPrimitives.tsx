"use client";

import Image from "next/image";
import { useRef, type ReactNode } from "react";
import Link from "@/i18n/Link";
import type { OverviewAction } from "@/i18n/public/ai-tools-en";
import styles from "./ToolsOverview.module.css";

export function OverviewPicture({ image, className, sizes = "(max-width: 767px) 82vw, 24vw", preload = false, decorative = false }: {
  image: { readonly file: string; readonly alt: string }; className?: string; sizes?: string; preload?: boolean; decorative?: boolean;
}) {
  return <Image src={`/assets/images/ai-tools/${image.file}`} alt={decorative ? "" : image.alt} fill sizes={sizes} preload={preload} className={className} />;
}

export function OverviewIcon({ name = "spark" }: { name?: string }) {
  return <svg aria-hidden="true" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
    {/video|cinematic|motion|lipsync/.test(name) ? <><rect x="3" y="6" width="13" height="12" rx="3" /><path d="m16 10 5-3v10l-5-3" /></> : /volume|audio|sound|music/.test(name) ? <><path d="M8 8v8m4-11v14m4-12v10m4-7v4M4 10v4" /></> : /microphone|voice/.test(name) ? <><rect x="9" y="3" width="6" height="12" rx="3" /><path d="M6 11v2a6 6 0 0 0 12 0v-2m-6 8v3" /></> : /image|camera|mockup/.test(name) ? <><rect x="3" y="4" width="18" height="16" rx="3" /><circle cx="8" cy="9" r="1.5" /><path d="m4 17 5-5 4 3 3-5 4 6" /></> : /spaces|flow|shuffle|grid|collection/.test(name) ? <><rect x="3" y="3" width="6" height="6" rx="2" /><rect x="15" y="15" width="6" height="6" rx="2" /><path d="M9 6h7a3 3 0 0 1 3 3v3M5 12v3a3 3 0 0 0 3 3h7" /></> : /user|assistant/.test(name) ? <><rect x="4" y="7" width="16" height="12" rx="5" /><path d="M12 3v4m-4 6h.01M16 13h.01M9 16h6" /></> : /3d|cube/.test(name) ? <><path d="m12 2 9 5v10l-9 5-9-5V7Zm0 0v20M3 7l9 5 9-5" /></> : /upscale|expand/.test(name) ? <><path d="M9 3H3v6m12-6h6v6M3 15v6h6m12-6v6h-6M3 3l6 6m12-6-6 6M3 21l6-6m12 6-6-6" /></> : <path d="m12 3 2.4 6.6L21 12l-6.6 2.4L12 21l-2.4-6.6L3 12l6.6-2.4Z" />}
  </svg>;
}

export function OverviewActionLink({ item, onPreview, className, children, ...rest }: {
  item: OverviewAction; onPreview: (item: OverviewAction) => void; className?: string; children: ReactNode;
  "data-tool-artwork"?: boolean; "data-audience-card"?: boolean;
}) {
  return item.href ? <Link href={item.href} className={className} {...rest}>{children}</Link> : <button type="button" className={className} onClick={() => onPreview(item)} {...rest}>{children}</button>;
}

export function OverviewTabs({ prefix, label, items, selected, onChange, className, orientation = "horizontal" }: {
  prefix: string; label: string; items: readonly { readonly id: string; readonly label: string }[]; selected: string;
  onChange: (id: string) => void; className?: string; orientation?: "horizontal" | "vertical";
}) {
  const refs = useRef<(HTMLButtonElement | null)[]>([]);
  const select = (index: number, focus = false) => {
    const item = items[index];
    onChange(item.id);
    if (focus) refs.current[index]?.focus();
    refs.current[index]?.scrollIntoView?.({ block: "nearest", inline: "nearest", behavior: "instant" });
  };
  return <div className={className} role="tablist" aria-label={label} aria-orientation={orientation}>
    {items.map((item,index) => <button type="button" role="tab" key={item.id} id={`${prefix}-tab-${item.id}`} aria-controls={`${prefix}-panel-${item.id}`} aria-selected={selected === item.id} tabIndex={selected === item.id ? 0 : -1} ref={element => { refs.current[index] = element; }} onClick={() => select(index)} onKeyDown={event => {
      let target: number | undefined;
      if (event.key === "ArrowRight" || event.key === "ArrowDown") target = (index+1)%items.length;
      if (event.key === "ArrowLeft" || event.key === "ArrowUp") target = (index-1+items.length)%items.length;
      if (event.key === "Home") target = 0;
      if (event.key === "End") target = items.length-1;
      if (target !== undefined) { event.preventDefault(); select(target,true); }
    }}>{item.label}</button>)}
  </div>;
}

export function OverviewHeading({ title, description, children }: { title: string; description: string; children?: ReactNode }) {
  return <div className={styles.sectionHeading}><div><h2>{title}</h2><p>{description}</p></div>{children}</div>;
}
