import Image from "next/image";
import type { ComponentPropsWithoutRef } from "react";

import { assetPaths } from "@/assets/asset-paths";

import styles from "./CreditAmount.module.css";

type CreditAmountProps = Omit<ComponentPropsWithoutRef<"span">, "children"> & {
  prefix?: string;
  value: number;
};

function getCreditUnit(value: number): string {
  const absoluteValue = Math.abs(value);
  const lastTwoDigits = absoluteValue % 100;
  const lastDigit = absoluteValue % 10;

  if (lastTwoDigits >= 11 && lastTwoDigits <= 14) {
    return "звёзд";
  }

  if (lastDigit === 1) {
    return "звезда";
  }

  if (lastDigit >= 2 && lastDigit <= 4) {
    return "звезды";
  }

  return "звёзд";
}

export function CreditAmount({
  "aria-label": ariaLabel,
  className,
  prefix,
  value,
  ...props
}: Readonly<CreditAmountProps>) {
  const accessibleLabel = [prefix, value, getCreditUnit(value)].filter(Boolean).join(" ");
  const classes = [styles.amount, className].filter(Boolean).join(" ");

  return (
    <span aria-label={ariaLabel ?? accessibleLabel} className={classes} {...props}>
      <span aria-hidden="true">{prefix ? `${prefix} ` : ""}{value}</span>
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
