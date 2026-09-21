"use client";

import type { ButtonHTMLAttributes } from "react";
import { Button } from "@/components/ui/Button/Button";
import { Tooltip } from "@/components/ui/Tooltip/Tooltip";
import { RetryUploadIcon } from "@/components/icons/RetryUploadIcon";
import styles from "./AsyncState.module.css";

export type RetryActionProps = Omit<ButtonHTMLAttributes<HTMLButtonElement>, "children"> & { label: string; iconOnly?: boolean };

export function RetryAction({ label, iconOnly = false, className, ...props }: RetryActionProps) {
  const button = <Button {...props} aria-label={props["aria-label"] ?? label} variant="outline"
    className={[styles.retry, iconOnly && styles.iconRetry, className].filter(Boolean).join(" ")}>
    <RetryUploadIcon />{iconOnly ? null : <span>{label}</span>}
  </Button>;
  return iconOnly ? <Tooltip label={label}>{button}</Tooltip> : button;
}
