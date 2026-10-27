"use client";

import { PasswordField } from "./PasswordField";

export function PasswordCreationField({ id, email, value, disabled, onChange }: {
  id: string; email: string; value: string; disabled?: boolean; onChange: (value: string) => void;
}) {
  return <PasswordField id={id} value={value} email={email} disabled={disabled} onChange={onChange} autoComplete="new-password" minLength={8} showStrength />;
}
