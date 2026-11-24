"use client";

import { Button } from "@/components/ui/Button/Button";
import { ModeSwitchPanel } from "@/components/ui/ModeSwitchPanel/ModeSwitchPanel";
import { PopoverSurface } from "@/components/ui/PopoverPanel/PopoverPanel";
import { RangeSlider } from "@/components/ui/RangeSlider/RangeSlider";
import { presentationExampleRu } from "@/i18n/presentation-example";
import { ColorField, readableInk, type PresentationColorValues } from "./PresentationColors";
import { fonts, presentationModule, type ModuleAppearance, type Slide } from "./presentation-modules";
import styles from "./PresentationExample.module.css";

const copy = presentationExampleRu.inspector;
const fontOptions = [{ id: "default", label: copy.commonFont }, ...fonts] as const;
const lightFills = { tableFill: "#f6f4ef", tableHeader: "#ffffff", tableHighlight: "#f5f2fe" };
const darkFills = { tableFill: "#151b23", tableHeader: "#353255", tableHighlight: "#4a4277" };

export function PresentationInspector({ slide, selected, saved, colors, light, onChange }: {
  slide: Slide;
  selected: string;
  saved?: ModuleAppearance;
  colors: PresentationColorValues;
  light: boolean;
  onChange: (value?: ModuleAppearance) => void;
}) {
  const target = presentationModule(slide, selected);
  const tableColor = target.tableColor;
  const isBlock = target.kind === "block";
  const canSetFill = isBlock || target.kind === "background";
  const canSetText = isBlock || target.kind === "text";
  const slideFill = colors.background ?? (light ? "#f6f4ef" : "#151b23");
  const usesSlideFill = slide === "security" || tableColor === "tableFill";
  const inheritedFill = tableColor
    ? colors[tableColor] ?? (usesSlideFill ? slideFill : (light ? lightFills : darkFills)[tableColor])
    : isBlock && !usesSlideFill ? (light ? "#ffffff" : "#353255") : slideFill;
  const inheritedInk = readableInk(saved?.fill ?? (tableColor && colors[tableColor]))
    ?? (!isBlock || usesSlideFill ? colors.text ?? readableInk(colors.background) : undefined)
    ?? (light ? "#0d1117" : "#ffffff");

  return <PopoverSurface className={styles.inspector} animated={false} role="region" aria-label={copy.ariaLabel}>
    <div className={styles.inspectorContent}>
      <div>
        <p className={styles.inspectorLabel}>{copy.ariaLabel}</p>
        <h3 aria-live="polite">{target.label}</h3>
      </div>
      <div className={styles.inspectorFields}>
        {canSetFill && <ColorField label={copy.fill} value={saved?.fill} fallback={inheritedFill} onChange={fill => onChange({ ...saved, fill })} />}
        {canSetText && <>
          <ColorField label={copy.textColor} value={saved?.ink} fallback={inheritedInk} onChange={ink => onChange({ ...saved, ink })} />
          <div className={styles.setting}>
            <span>{copy.font}</span>
            <ModeSwitchPanel className={styles.fontOptions} ariaLabel={copy.elementFont} items={fontOptions} activeID={saved?.font ?? "default"} onChange={id => onChange({ ...saved, font: id === "default" ? undefined : id })} density="compact" fullWidth />
          </div>
        </>}
        {target.kind === "image" && <div className={styles.setting}>
          <span>{copy.opacity} · {saved?.opacity ?? 100}%</span>
          <RangeSlider aria-label={copy.illustrationOpacity} min={0} max={100} value={saved?.opacity ?? 100} onValueChange={opacity => onChange({ opacity })} />
        </div>}
      </div>
      <div className={styles.settingsFooter}>
        <span>{target.kind === "background" ? copy.selectElement : copy.free}</span>
        <Button variant="outline" disabled={!saved} onClick={() => onChange()}>{copy.reset}</Button>
      </div>
    </div>
  </PopoverSurface>;
}
