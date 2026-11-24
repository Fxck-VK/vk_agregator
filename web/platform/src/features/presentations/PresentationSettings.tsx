"use client";

import { useRef, useState } from "react";
import { Button } from "@/components/ui/Button/Button";
import { ModeSwitchPanel } from "@/components/ui/ModeSwitchPanel/ModeSwitchPanel";
import { PopoverPanel } from "@/components/ui/PopoverPanel/PopoverPanel";
import { presentationExampleRu } from "@/i18n/presentation-example";
import { PresentationColors, parsePresentationColors, type PresentationColorValues } from "./PresentationColors";
import { fonts, parsePresentationModules, type PresentationFont, type PresentationModuleValues } from "./presentation-modules";
import styles from "./PresentationExample.module.css";

const copy = presentationExampleRu.settings;
const backgrounds = copy.backgrounds;
const animationModes = copy.animationModes;
const sections = copy.sections;
const storageKey = "epifera.presentation.appearance";

export type PresentationAppearance = {
  font: PresentationFont;
  background: (typeof backgrounds)[number]["id"];
  animation: (typeof animationModes)[number]["id"];
  colors: PresentationColorValues;
  modules: PresentationModuleValues;
};

const defaults: PresentationAppearance = { font: "geist", background: "original", animation: "on", colors: {}, modules: {} };

export function readPresentationAppearance(): PresentationAppearance {
  try {
    const saved: unknown = JSON.parse(window.localStorage.getItem(storageKey) ?? "null");
    if (!saved || typeof saved !== "object") return defaults;
    return {
      font: fonts.find(option => "font" in saved && option.id === saved.font)?.id ?? defaults.font,
      background: backgrounds.find(option => "background" in saved && option.id === saved.background)?.id ?? defaults.background,
      animation: animationModes.find(option => "animation" in saved && option.id === saved.animation)?.id ?? defaults.animation,
      colors: parsePresentationColors("colors" in saved ? saved.colors : undefined),
      modules: parsePresentationModules("modules" in saved ? saved.modules : undefined),
    };
  } catch {
    return defaults;
  }
}

export function savePresentationAppearance(appearance: PresentationAppearance) {
  try {
    window.localStorage.setItem(storageKey, JSON.stringify(appearance));
  } catch {
    // The current viewer still works when browser storage is unavailable.
  }
}

export function PresentationSettings({ value, table, lightByDefault, onChange }: { value: PresentationAppearance; table: boolean; lightByDefault: boolean; onChange: (value: PresentationAppearance) => void }) {
  const trigger = useRef<HTMLButtonElement>(null);
  const [open, setOpen] = useState(false);
  const [section, setSection] = useState<(typeof sections)[number]["id"]>("show");
  const light = value.background === "light" || value.background === "original" && lightByDefault;

  return <>
    <Button ref={trigger} className={styles.settingsTrigger} variant="outline" aria-label={copy.ariaLabel} aria-haspopup="dialog" aria-expanded={open} onClick={() => setOpen(current => !current)}>
      <svg aria-hidden="true" viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round"><path d="M4 6h5m4 0h7M4 12h9m4 0h3M4 18h3m4 0h9"/><circle cx="11" cy="6" r="2"/><circle cx="15" cy="12" r="2"/><circle cx="9" cy="18" r="2"/></svg>
      <span>{copy.trigger}</span>
    </Button>
    <PopoverPanel label={copy.ariaLabel} anchorRef={trigger} isOpen={open} onClose={() => setOpen(false)} align="end" width={340} portalLayer={170}>
      <div className={styles.settings}>
        <p className={styles.settingsTitle}>{copy.ariaLabel}</p>
        <ModeSwitchPanel ariaLabel={copy.sectionLabel} items={sections} activeID={section} onChange={setSection} density="compact" fullWidth />
        {section === "colors" ? <PresentationColors value={value.colors} table={table} light={light} onChange={colors => onChange({ ...value, colors })} /> : <>
          <div className={styles.setting}><span>{copy.font}</span><ModeSwitchPanel ariaLabel={copy.font} items={fonts} activeID={value.font} onChange={font => onChange({ ...value, font })} density="compact" fullWidth /></div>
          <div className={styles.setting}><span>{copy.background}</span><ModeSwitchPanel ariaLabel={copy.background} items={backgrounds} activeID={value.background} onChange={background => onChange({ ...value, background })} density="compact" fullWidth /></div>
          <div className={styles.setting}><span>{copy.animation}</span><ModeSwitchPanel ariaLabel={copy.animation} items={animationModes} activeID={value.animation} onChange={animation => onChange({ ...value, animation })} density="compact" fullWidth /></div>
        </>}
        <div className={styles.settingsFooter}><span>{copy.free}</span><Button variant="outline" onClick={() => onChange({ ...defaults, modules: value.modules })}>{copy.reset}</Button></div>
      </div>
    </PopoverPanel>
  </>;
}
