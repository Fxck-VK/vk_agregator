import type { ComponentPropsWithoutRef } from "react";

import styles from "./InputSurface.module.css";

type InputSurfaceProps = ComponentPropsWithoutRef<"div">;

export function InputSurface({ className, ...props }: InputSurfaceProps) {
  const classNames = [styles.surface, className].filter(Boolean).join(" ");

  return <div {...props} className={classNames} data-ui="input-surface" />;
}
