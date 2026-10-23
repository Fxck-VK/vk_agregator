"use client";

import { useEffect, useRef, useState } from "react";
import { BrandLogo } from "@/components/brand/BrandLogo/BrandLogo";
import { BrandName } from "@/components/brand/BrandName/BrandName";
import { brandName } from "@/config/brand";
import type { Dictionary } from "@/i18n/dictionary";
import Link from "@/i18n/Link";
import { showcase } from "./marketing-content";
import { MarketingSearch } from "./MarketingSearch";
import styles from "./MarketingHome.module.css";

export function MarketingHeader({ t }: { t: Dictionary["marketing"] }) {
  const [open, setOpen] = useState(false);
  const [mobileOpen, setMobileOpen] = useState(false);
  const [searchOpen, setSearchOpen] = useState(false);
  const header = useRef<HTMLElement>(null);
  const trigger = useRef<HTMLButtonElement>(null);
  const mobileTrigger = useRef<HTMLButtonElement>(null);
  const hoverOpened = useRef(false);

  const closeMenus = () => { setOpen(false); setMobileOpen(false); hoverOpened.current = false; };
  useEffect(() => {
    if (!open && !mobileOpen) return;
    const escape = (event: globalThis.KeyboardEvent) => {
      if (event.key !== "Escape" || document.querySelector('[role="dialog"]')) return;
      event.preventDefault(); setOpen(false); setMobileOpen(false); hoverOpened.current = false;
      (mobileOpen ? mobileTrigger : trigger).current?.focus();
    };
    window.addEventListener("keydown", escape);
    return () => window.removeEventListener("keydown", escape);
  }, [open, mobileOpen]);
  useEffect(() => {
    const shortcut = (event: globalThis.KeyboardEvent) => {
      if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === "k") {
        event.preventDefault();
        if (document.querySelector('[role="dialog"]')) return;
        closeMenus(); setSearchOpen(true);
      }
    };
    window.addEventListener("keydown", shortcut);
    return () => window.removeEventListener("keydown", shortcut);
  }, []);
  useEffect(() => {
    if (!mobileOpen || !window.matchMedia) return;
    const query = window.matchMedia("(max-width: 600px)");
    const previous = document.body.style.overflow;
    const background = Array.from(document.querySelectorAll<HTMLElement>("#public-main, footer"));
    const previousInert = background.map(element => element.inert);
    const update = () => {
      document.body.style.overflow = query.matches ? "hidden" : previous;
      background.forEach((element, index) => { element.inert = query.matches || previousInert[index]; });
    };
    update(); query.addEventListener("change", update);
    return () => { document.body.style.overflow = previous; background.forEach((element, index) => { element.inert = previousInert[index]; }); query.removeEventListener("change", update); };
  }, [mobileOpen]);

  useEffect(() => {
    if (!open && !mobileOpen) return;
    const closeOutside = (event: PointerEvent) => {
      if (event.target instanceof Node && !header.current?.contains(event.target)) {
        setOpen(false); setMobileOpen(false); hoverOpened.current = false;
      }
    };
    document.addEventListener("pointerdown", closeOutside);
    return () => document.removeEventListener("pointerdown", closeOutside);
  }, [open, mobileOpen]);

  return (
    <header className={styles.header} ref={header} data-mobile-open={mobileOpen || undefined} onPointerLeave={event => { if (event.pointerType === "mouse" && !mobileOpen) { setOpen(false); hoverOpened.current = false; } }} onKeyDown={event => {
      if (searchOpen) return;
      if (event.key === "Tab" && mobileOpen && window.matchMedia?.("(max-width: 600px)").matches) {
        const controls = Array.from(event.currentTarget.querySelectorAll<HTMLElement>("a[href],button:not(:disabled)")).filter(element => element.getClientRects().length && getComputedStyle(element).visibility !== "hidden");
        if (event.shiftKey && document.activeElement === controls[0]) { event.preventDefault(); controls.at(-1)?.focus(); }
        else if (!event.shiftKey && document.activeElement === controls.at(-1)) { event.preventDefault(); controls[0]?.focus(); }
      }
    }}>
      <Link href="/" className={styles.brand} aria-label={brandName}><BrandLogo size={32} /><BrandName /></Link>
      <button ref={mobileTrigger} className={styles.mobileMenuButton} type="button" aria-label={mobileOpen ? t.close : t.menu} aria-expanded={mobileOpen} aria-controls="marketing-navigation" onClick={() => { setMobileOpen(!mobileOpen); setOpen(false); }}><span /><span /></button>
      <nav className={styles.navigation} id="marketing-navigation" data-open={mobileOpen || undefined} aria-label={t.creativeSuite}>
        <button ref={trigger} type="button" className={styles.navigationTrigger} aria-expanded={open} aria-controls="creative-suite-menu" onPointerEnter={event => { if (event.pointerType === "mouse" && !open) { hoverOpened.current = true; setOpen(true); } }} onClick={() => { setOpen(hoverOpened.current || !open); hoverOpened.current = false; }}>{t.creativeSuite}<span aria-hidden="true">⌄</span></button>
      {open ? <div id="creative-suite-menu" className={styles.megaMenu}>
        <div className={styles.menuLinks}>{showcase.map((item, index) => <Link key={item.href} href={item.href} onClick={() => { setOpen(false); setMobileOpen(false); }}><span aria-hidden="true">{item.icon}</span><span><strong>{t.categoryTabs[index]}</strong><small>{t.categoryDescriptions[index]}</small></span><span aria-hidden="true">↗</span></Link>)}</div>
        <Link className={styles.menuFeature} href="/app/inspiration" onClick={closeMenus}><span>{t.originalsAction} ↗</span></Link>
      </div> : null}
        <Link href="#tools" onClick={closeMenus}>{t.resources}</Link><Link href="#plans" onClick={closeMenus}>{t.business}</Link><Link href="#workflow" onClick={closeMenus}>{t.pricing}</Link>
      </nav>
      <button type="button" className={styles.search} onClick={() => { closeMenus(); setSearchOpen(true); }} aria-label={t.search}><svg aria-hidden="true" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8"><circle cx="10.5" cy="10.5" r="6.5"/><path d="m16 16 4.5 4.5"/></svg><span>{t.search}</span><kbd aria-hidden="true">{t.searchShortcut}</kbd></button>
      <div className={styles.headerActions}><Link href="/login" className={styles.login}>{t.login}</Link><Link className={styles.signup} href="/login">{t.signup}</Link></div>
      {searchOpen ? <MarketingSearch onClose={() => setSearchOpen(false)} /> : null}
    </header>
  );
}
