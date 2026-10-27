"use client";

import { type KeyboardEvent, useEffect, useId, useRef } from "react";
import { useDictionary } from "@/i18n/LocaleProvider";
import styles from "./VerificationCodeField.module.css";

type VerificationCodeFieldProps = {
  id?: string;
  label?: string;
  /** Partial values may contain spaces for empty middle cells; a complete value is six digits. */
  value: string;
  onChange: (value: string) => void;
  onComplete?: (value: string) => void;
  disabled?: boolean;
  invalid?: boolean;
  error?: string;
};

export function VerificationCodeField({ id, label, value, onChange, onComplete, disabled = false, invalid = false, error }: VerificationCodeFieldProps) {
  const t = useDictionary();
  const generatedId = useId();
  const fieldId = id ?? generatedId;
  const caption = label ?? t.auth.code;
  const inputs = useRef<(HTMLInputElement | null)[]>([]);
  const lastValue = useRef(value);
  const completed = useRef<string | null>(null);
  const completion = useRef(onComplete);
  const digits = Array.from({ length: 6 }, (_, index) => /^\d$/.test(value[index] ?? "") ? value[index] : "");
  useEffect(() => { completion.current = onComplete; }, [onComplete]);
  useEffect(() => {
    if (lastValue.current !== value) { lastValue.current = value; completed.current = null; }
    if (!/^\d{6}$/.test(value)) return;
    // A manual submit can set pending before the delay ends; it also consumes
    // automatic submission of this unchanged code when the request finishes.
    if (disabled) { completed.current = value; return; }
    if (completed.current === value) return;
    const timer = window.setTimeout(() => { completed.current = value; completion.current?.(value); }, 1000);
    return () => window.clearTimeout(timer);
  }, [value, disabled]);

  function write(next: string[]) { onChange(next.map(digit => digit || " ").join("").trimEnd()); }
  function fill(text: string, index: number) {
    if (!/^\d*$/.test(text) || text.length > 6) return;
    if (text.length > 1) {
      const start = text.length === 6 ? 0 : index;
      const next = [...digits];
      text.slice(0, 6 - start).split("").forEach((digit, offset) => { next[start + offset] = digit; });
      write(next); inputs.current[Math.min(5, start + text.length)]?.focus();
      return;
    }
    const next = [...digits]; next[index] = text; write(next);
    if (text && index < 5) inputs.current[index + 1]?.focus();
  }
  function keyDown(event: KeyboardEvent<HTMLInputElement>, index: number) {
    const nextIndex = event.key === "ArrowLeft" ? Math.max(0, index - 1) : event.key === "ArrowRight" ? Math.min(5, index + 1) : event.key === "Home" ? 0 : event.key === "End" ? 5 : null;
    if (nextIndex !== null) { event.preventDefault(); inputs.current[nextIndex]?.focus(); return; }
    if (event.key === "Backspace" || event.key === "Delete") {
      event.preventDefault(); const next = [...digits]; next[index] = ""; write(next);
      if (event.key === "Backspace" && index > 0) inputs.current[index - 1]?.focus();
    }
  }
  return <fieldset className={styles.field} disabled={disabled} aria-describedby={error ? `${fieldId}-error` : undefined}>
    <legend className={styles.label}>{caption}</legend>
    <div className={styles.digits}>
      {digits.map((digit, index) => <input key={index} ref={element => { inputs.current[index] = element; }}
        id={`${fieldId}-${index + 1}`} name={`${fieldId}-${index + 1}`} className={styles.digit} type="text" inputMode="numeric"
        autoComplete={index === 0 ? "one-time-code" : "off"} maxLength={6} pattern="[0-9]*" value={digit} disabled={disabled}
        aria-label={`${caption}: ${index + 1}`} aria-invalid={invalid || Boolean(error) || undefined} aria-describedby={error ? `${fieldId}-error` : undefined}
        onFocus={event => event.currentTarget.select()} onKeyDown={event => keyDown(event, index)}
        onChange={event => fill(event.target.value, index)} onPaste={event => { event.preventDefault(); fill(event.clipboardData.getData("text").trim(), index); }} />)}
    </div>
    {error ? <p id={`${fieldId}-error`} role="alert" className={styles.error}>{error}</p> : null}
  </fieldset>;
}
