import type { ComponentProps } from "react";
import { InputSurface } from "@/components/ui/InputSurface/InputSurface";
import styles from "./LoginForm/LoginForm.module.css";

export function CredentialField({ label, hint, ...props }: ComponentProps<"input"> & { label: string; hint?: string }) {
  return <div className={styles.field}>
    <label htmlFor={props.id}>{label}</label>
    <InputSurface className={styles.inputSurface}><input {...props} aria-describedby={hint ? `${props.id}-hint` : undefined} /></InputSurface>
    {hint ? <small id={`${props.id}-hint`}>{hint}</small> : null}
  </div>;
}
