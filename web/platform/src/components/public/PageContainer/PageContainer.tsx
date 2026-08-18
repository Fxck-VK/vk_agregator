import type { HTMLAttributes } from "react";

import styles from "./PageContainer.module.css";

type PageContainerProps = HTMLAttributes<HTMLDivElement> & {
  size?: "narrow" | "content" | "wide";
};

export function PageContainer({ children, className, size = "content", ...props }: PageContainerProps) {
  const classes = [styles.container, className].filter(Boolean).join(" ");

  return (
    <div className={classes} data-size={size} {...props}>
      {children}
    </div>
  );
}
