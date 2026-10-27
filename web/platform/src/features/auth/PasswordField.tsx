"use client";

import { type ComponentProps, useState } from "react";
import { useDictionary } from "@/i18n/LocaleProvider";
import { CredentialField } from "./CredentialField";
import { PasswordStrengthMeter } from "./PasswordStrengthMeter";
import styles from "./LoginForm/LoginForm.module.css";

type PasswordFieldProps = Omit<ComponentProps<"input">, "type" | "value" | "onChange" | "id"> & {
  id: string;
  value: string;
  onChange: (value: string) => void;
  label?: string;
  hint?: string;
  email?: string;
  showStrength?: boolean;
};

export function PasswordField({ id, value, onChange, label, hint, email = "", showStrength = false,
  autoComplete = "current-password", name = "password", required = true, maxLength = 256,
  onBlur, onKeyDown, onKeyUp, ...props }: PasswordFieldProps) {
  const t = useDictionary();
  const [visible, setVisible] = useState(false);
  const [capsLock, setCapsLock] = useState(false);
  const describedBy = [props["aria-describedby"], showStrength && value ? `${id}-strength` : "", capsLock ? `${id}-caps-lock` : ""].filter(Boolean).join(" ") || undefined;
  return <div className={styles.passwordCreation}>
    <CredentialField {...props} id={id} label={label ?? t.login.passwordLabel} hint={hint} name={name}
      type={visible ? "text" : "password"} autoComplete={autoComplete} maxLength={maxLength} required={required} value={value}
      aria-describedby={describedBy} onChange={event => onChange(event.target.value)}
      onKeyDown={event => { setCapsLock(event.getModifierState("CapsLock")); onKeyDown?.(event); }}
      onKeyUp={event => { setCapsLock(event.getModifierState("CapsLock")); onKeyUp?.(event); }}
      onBlur={event => { setCapsLock(false); onBlur?.(event); }}
      trailingAction={<button type="button" className={styles.revealPassword} disabled={props.disabled} aria-label={visible ? t.auth.hidePassword : t.auth.showPassword} aria-pressed={visible} onClick={() => setVisible(current => !current)}>
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" focusable="false">
          <path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12Z" /><circle cx="12" cy="12" r="3" />{visible ? null : <path d="m3 3 18 18" />}
        </svg>
      </button>} />
    {capsLock ? <p id={`${id}-caps-lock`} className={styles.capsLockHint} role="status">{t.auth.passwordCapsLock}</p> : null}
    {showStrength ? <PasswordStrengthMeter password={value} email={email} id={`${id}-strength`} /> : null}
  </div>;
}
