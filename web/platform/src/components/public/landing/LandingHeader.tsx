"use client";

import { useEffect, useRef, useState } from "react";
import { BrandLogo } from "@/components/brand/BrandLogo/BrandLogo";
import { BrandName } from "@/components/brand/BrandName/BrandName";
import { brandName } from "@/config/brand";
import Link from "@/i18n/Link";
import { LandingArrow } from "./LandingPrimitives";
import styles from "./LandingChrome.module.css";

export type LandingNavigationItem = {
  readonly label: string;
  readonly href?: string;
  readonly groups?: readonly { readonly title: string; readonly links: readonly { readonly label: string; readonly href: string }[] }[];
};
type Labels = { openMenu: string; closeMenu: string; navigation: string; search: string; login: string; signup: string };

export function LandingHeader({ navigation, labels, onSearch, variant = "overlay", siteAction }: { navigation: readonly LandingNavigationItem[]; labels: Labels; onSearch: () => void; variant?: "overlay" | "editorial"; siteAction?: { readonly label: string; readonly href: string } }) {
  const [open, setOpen] = useState<number | null>(null);
  const [mobileOpen, setMobileOpen] = useState(false);
  const header = useRef<HTMLElement>(null);
  const mobileTrigger = useRef<HTMLButtonElement>(null);
  const triggers = useRef<(HTMLButtonElement | null)[]>([]);
  const hoverOpened = useRef<number | null>(null);
  const close = () => { hoverOpened.current = null; setOpen(null); setMobileOpen(false); };
  const search = () => { close(); onSearch(); };

  useEffect(() => {
    const key = (event: KeyboardEvent) => {
      if (document.querySelector('[role="dialog"]')) return;
      if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === "k") { event.preventDefault(); setOpen(null); setMobileOpen(false); onSearch(); }
      if (event.key === "Escape" && (mobileOpen || open !== null)) {
        event.preventDefault(); setOpen(null); setMobileOpen(false);
        (mobileOpen ? mobileTrigger.current : triggers.current[open!])?.focus();
      }
    };
    const outside = (event: PointerEvent) => { if (event.target instanceof Node && !header.current?.contains(event.target)) { setOpen(null); setMobileOpen(false); } };
    window.addEventListener("keydown", key); document.addEventListener("pointerdown", outside);
    return () => { window.removeEventListener("keydown", key); document.removeEventListener("pointerdown", outside); };
  }, [mobileOpen, open, onSearch]);

  useEffect(() => {
    if (!mobileOpen || !window.matchMedia?.("(max-width: 1050px)").matches) return;
    const previous = document.body.style.overflow;
    const background = Array.from(document.querySelectorAll<HTMLElement>("#public-main,[data-landing-footer]"));
    const previousInert = background.map(element => element.inert);
    document.body.style.overflow = "hidden"; background.forEach(element => { element.inert = true; });
    const trap = (event: KeyboardEvent) => {
      if (event.key !== "Tab") return;
      const elements = Array.from(header.current?.querySelectorAll<HTMLElement>('button,a[href]') ?? []).filter(element => element.getClientRects().length > 0);
      const first = elements[0]; const last = elements.at(-1);
      if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last?.focus(); }
      if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first?.focus(); }
    };
    window.addEventListener("keydown", trap);
    return () => { document.body.style.overflow = previous; background.forEach((element, index) => { element.inert = previousInert[index]; }); window.removeEventListener("keydown", trap); };
  }, [mobileOpen]);

  return <header ref={header} className={styles.header} data-variant={variant === "editorial" ? variant : undefined} data-mobile-open={mobileOpen || undefined} data-menu-open={open !== null || undefined}>
    <div className={styles.headerBar}>
      <button type="button" ref={mobileTrigger} className={styles.mobileTrigger} aria-label={mobileOpen ? labels.closeMenu : labels.openMenu} aria-expanded={mobileOpen} aria-controls="landing-navigation" onClick={() => { setMobileOpen(!mobileOpen); setOpen(null); }}><span /><span /><span /></button>
      <Link href="/" className={styles.brand} aria-label={brandName}><BrandLogo size={33} /><BrandName /></Link>
      <nav id="landing-navigation" aria-label={labels.navigation} className={styles.navigation}>
        {navigation.map((item, index) => <div className={styles.navItem} key={item.label} onMouseEnter={() => {
          if (!mobileOpen && item.groups && open !== index && window.matchMedia?.("(hover: hover)").matches) { hoverOpened.current = index; setOpen(index); }
        }}>
          {item.groups ? <><button type="button" ref={node => { triggers.current[index] = node; }} aria-expanded={open === index} aria-controls={`landing-menu-${index}`} onClick={() => {
            const openedByHover = hoverOpened.current === index;
            hoverOpened.current = null;
            setOpen(openedByHover ? index : open === index ? null : index);
          }} className={styles.navButton}>{item.label}<span className={styles.chevron} aria-hidden="true">⌄</span></button>
            {open === index ? <div id={`landing-menu-${index}`} className={styles.megaMenu} onMouseLeave={() => { if (!mobileOpen) { hoverOpened.current = null; setOpen(null); } }}>
              <div className={styles.menuGroups}>{item.groups.map(group => <div key={group.title}><p>{group.title}</p>{group.links.map(link => <Link href={link.href} key={link.label} onClick={close}>{link.label}<LandingArrow /></Link>)}</div>)}</div>
              <Link href="/app" className={styles.menuCta} onClick={close}><span>{labels.signup}</span><LandingArrow /></Link>
            </div> : null}</> : <Link href={item.href!} onClick={close} className={styles.navButton}>{item.label}</Link>}
        </div>)}
      </nav>
      <div className={styles.actions}><button type="button" className={styles.searchTrigger} onClick={search} aria-label={labels.search}><svg aria-hidden="true" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8"><circle cx="10.5" cy="10.5" r="6.5" /><path d="m16 16 5 5" /></svg><span>{labels.search}</span></button>{variant === "editorial" && siteAction ? <Link href={siteAction.href} className={styles.siteAction}>{siteAction.label}</Link> : <><Link href="/login" className={styles.login}>{labels.login}</Link><Link href="/login?mode=register" className={styles.signup}>{labels.signup}</Link></>}</div>
    </div>
  </header>;
}
