import type { ComponentProps, ReactNode } from "react";
import { InputSurface } from "@/components/ui/InputSurface/InputSurface";
import styles from "./LoginForm/LoginForm.module.css";

export function CredentialField({ label, hint, trailingAction, ...props }: ComponentProps<"input"> & { label: string; hint?: string; trailingAction?: ReactNode }) {
  const describedBy = [props["aria-describedby"], hint ? `${props.id}-hint` : undefined].filter(Boolean).join(" ") || undefined;
  return <div className={styles.field}>
    <label htmlFor={props.id}>{label}</label>
    <InputSurface className={styles.inputSurface}><input {...props} aria-describedby={describedBy} />{trailingAction}</InputSurface>
    {hint ? <small id={`${props.id}-hint`}>{hint}</small> : null}
  </div>;
}
