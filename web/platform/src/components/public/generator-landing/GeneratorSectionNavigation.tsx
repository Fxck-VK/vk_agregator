"use client";

import { useEffect, useRef, useState } from "react";
import styles from "./GeneratorLanding.module.css";

export function GeneratorSectionNavigation({ sections, label }: { sections: readonly { readonly id:string; readonly label:string }[]; label:string }) {
  const [active,setActive] = useState(sections[0].id);
  const chips = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if(typeof IntersectionObserver === "undefined") return;
    const visible = new Set<string>();
    const observer = new IntersectionObserver(entries => {
      entries.forEach(entry => { if(entry.isIntersecting) visible.add(entry.target.id); else visible.delete(entry.target.id); });
      const first = sections.find(item => visible.has(item.id)); if(first) setActive(first.id);
    },{rootMargin:"-15% 0px -55% 0px",threshold:0});
    sections.forEach(item => { const node=document.getElementById(item.id); if(node) observer.observe(node); });
    return () => observer.disconnect();
  },[sections]);
  useEffect(() => {
    const node=chips.current,item=node?.querySelector<HTMLElement>(`[href="#${active}"]`);
    if(node&&item) node.scrollTo?.({left:item.offsetLeft-node.clientWidth/2+item.clientWidth/2,behavior:"instant"});
  },[active]);
  return <><nav className={styles.rail} aria-label={label}><p className={styles.railTitle}>{label}</p><ol className={styles.railList}>{sections.map((item,index)=><li key={item.id}><a href={`#${item.id}`} aria-current={active===item.id?"location":undefined} className={`${styles.railLink} ${active===item.id?styles.railLinkActive:""}`} onClick={()=>setActive(item.id)}><span className={styles.menuIcon} aria-hidden="true">{["☷","◉","⚑","◧","✦","▥","▦"][index]}</span>{item.label}</a></li>)}</ol></nav><nav className={styles.chips} aria-label={label}><div ref={chips} className={styles.chipsTrack}>{sections.map(item=><a key={item.id} href={`#${item.id}`} aria-current={active===item.id?"location":undefined} className={`${styles.chip} ${active===item.id?styles.chipActive:""}`} onClick={()=>setActive(item.id)}>{item.label}</a>)}</div></nav></>;
}
