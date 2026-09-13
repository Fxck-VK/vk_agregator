import type { ComponentPropsWithoutRef } from "react";

import styles from "./MasonryGrid.module.css";

type MasonryGridProps = ComponentPropsWithoutRef<"ol">;

export function MasonryGrid({ className, ...props }: Readonly<MasonryGridProps>) {
  return (
    <ol
      {...props}
      className={[styles.grid, className].filter(Boolean).join(" ")}
    />
  );
}
