"use client";

import { useDictionary } from "@/i18n/LocaleProvider";
import { passwordStrength } from "@/lib/auth/password-strength";
import { isPasswordTooLong } from "@/lib/auth/password";
import styles from "./PasswordStrengthMeter.module.css";

export function PasswordStrengthMeter({ password, email, id }: { password: string; email?: string; id: string }) {
  const { auth: t } = useDictionary();
  const level = passwordStrength(password, email);
  if (!level) return null;
  const label = ["", t.passwordWeak, t.passwordMedium, t.passwordStrong][level];
  const hint = isPasswordTooLong(password) ? t.passwordTooLong : password.length < 8 ? t.passwordHint
    : level === 1 ? t.passwordStrengthWeakHint : level === 2 ? t.passwordStrengthMediumHint : t.passwordStrengthStrongHint;
  return <div className={styles.meter} data-level={level} id={id}>
    <div className={styles.track} role="meter" aria-label={t.passwordStrengthLabel} aria-valuemin={0} aria-valuemax={3} aria-valuenow={level} aria-valuetext={label}>
      {[1, 2, 3].map(segment => <span key={segment} data-filled={segment <= level} />)}
    </div>
    <p className={styles.label} role="status">{label}</p>
    <p className={styles.hint}>{hint}</p>
  </div>;
}
