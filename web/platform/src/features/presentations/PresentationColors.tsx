"use client";

import { useState, type CSSProperties } from "react";
import { InputSurface } from "@/components/ui/InputSurface/InputSurface";
import { presentationExampleRu } from "@/i18n/presentation-example";
import { normalizeColor, readableInk } from "./presentation-colors";
import styles from "./PresentationExample.module.css";

export { normalizeColor, readableInk } from "./presentation-colors";

const copy = presentationExampleRu.colors;
const fields = copy.fields;

export type ColorKey = (typeof fields)[number]["key"];
export type PresentationColorValues = Partial<Record<ColorKey, string>>;
export type PresentationColorFallbacks = { background: string; text: string };

export function parsePresentationColors(value: unknown): PresentationColorValues {
  const colors: PresentationColorValues = {};
  if (!value || typeof value !== "object") return colors;
  for (const { key } of fields) {
    const color = normalizeColor((value as Record<string, unknown>)[key]);
    if (color) colors[key] = color;
  }
  return colors;
}

export function presentationColorStyles(colors: PresentationColorValues): CSSProperties {
  return {
    "--custom-slide-background": colors.background,
    "--custom-slide-ink": colors.text ?? readableInk(colors.background),
    "--custom-table-fill": colors.tableFill,
    "--custom-table-ink": readableInk(colors.tableFill),
    "--custom-table-header": colors.tableHeader,
    "--custom-table-header-ink": readableInk(colors.tableHeader),
    "--custom-table-highlight": colors.tableHighlight,
    "--custom-table-highlight-ink": readableInk(colors.tableHighlight),
  } as CSSProperties;
}

export function ColorField({ label, value, fallback, onChange }: { label: string; value?: string; fallback: string; onChange: (color?: string) => void }) {
  const [draft, setDraft] = useState<string | null>(null);
  const validDraft = normalizeColor(draft);

  return <div className={styles.colorRow}>
    <span>{label}</span>
    <input className={styles.colorPicker} type="color" aria-label={`${copy.selectPrefix}: ${label}`} value={value ?? fallback} onChange={event => onChange(normalizeColor(event.currentTarget.value))} />
    <InputSurface className={styles.colorHex}>
      <input type="text" aria-label={`${copy.hexPrefix}: ${label}`} value={draft ?? value ?? ""} placeholder={copy.automatic} maxLength={7} spellCheck={false} autoComplete="off"
        aria-invalid={!!draft?.trim() && !validDraft} title={copy.hexHelp}
        onChange={event => setDraft(event.currentTarget.value)}
        onBlur={() => {
          if (draft !== null && (validDraft || !draft.trim())) onChange(validDraft);
          setDraft(null);
        }}
        onKeyDown={event => { if (event.key === "Enter") event.currentTarget.blur(); }} />
    </InputSurface>
  </div>;
}

export function PresentationColors({ value, table, light, colorFallbacks, onChange }: {
  value: PresentationColorValues;
  table: boolean;
  light: boolean;
  colorFallbacks?: PresentationColorFallbacks;
  onChange: (colors: PresentationColorValues) => void;
}) {
  return <div className={styles.colorRows}>
    {fields.slice(0, table ? fields.length : 2).map(field => {
      const { key, label } = field;
      const fallback = key === "background" || key === "text" ? colorFallbacks?.[key] : undefined;
      return <ColorField key={key} label={label} value={value[key]} fallback={fallback ?? (light ? field.light : field.dark)}
        onChange={color => onChange({ ...value, [key]: color })} />;
    })}
  </div>;
}
