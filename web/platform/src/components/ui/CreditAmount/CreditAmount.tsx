"use client";

import { countLabel } from "@/i18n/counts";

import { useMessages } from "@/i18n/LocaleProvider";
import { getTranslator, type Translator } from "@/i18n/messages";

import Image from "next/image";
import type { ComponentPropsWithoutRef } from "react";

import { assetPaths } from "@/assets/asset-paths";

import styles from "./CreditAmount.module.css";

type CreditAmountProps = Omit<ComponentPropsWithoutRef<"span">, "children"> & {
  prefix?: string;
  value: number;
};


export function getCreditAmountLabel(value: number, prefix?: string, msg: Translator = getTranslator("ru")): string {
  return [prefix, countLabel(msg, "stars", value)].filter(Boolean).join(" ");
}

export function CreditAmount({
  "aria-label": ariaLabel,
  className,
  prefix,
  value,
  ...props
}: Readonly<CreditAmountProps>) {
  const msg = useMessages();
  const accessibleLabel = getCreditAmountLabel(value, prefix, msg);
  const classes = [styles.amount, className].filter(Boolean).join(" ");

  return (
    <span aria-label={ariaLabel ?? accessibleLabel} className={classes} {...props}>
      <span aria-hidden="true" className={styles.value}>{prefix ? `${prefix} ` : ""}{new Intl.NumberFormat(msg.locale).format(value)}</span>
      <Image
        alt=""
        aria-hidden="true"
        className={styles.icon}
        data-testid="credit-star-icon"
        height={32}
        src={assetPaths.images.credits.star}
        width={32}
      />
    </span>
  );
}
