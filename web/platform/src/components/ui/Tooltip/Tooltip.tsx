import type { CSSProperties, ReactNode } from "react";

import styles from "./Tooltip.module.css";

type TooltipPlacement = "bottom" | "top";

type TooltipProps = {
  children: ReactNode;
  label: ReactNode;
  placement?: TooltipPlacement;
};

type TooltipBubbleProps = {
  children: ReactNode;
  className?: string;
  style?: CSSProperties;
};

export function Tooltip({ children, label, placement = "top" }: Readonly<TooltipProps>) {
  return (
    <span className={styles.anchor} data-placement={placement} data-ui="tooltip">
      {children}
      <TooltipBubble className={styles.anchoredBubble}>{label}</TooltipBubble>
    </span>
  );
}

export function TooltipBubble({ children, className, style }: Readonly<TooltipBubbleProps>) {
  return (
    <span
      className={className === undefined ? styles.bubble : `${styles.bubble} ${className}`}
      data-ui="tooltip-bubble"
      role="tooltip"
      style={style}
    >
      {children}
    </span>
  );
}
