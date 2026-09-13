import type { HTMLAttributes } from "react";

import styles from "./WorkspacePageFrame.module.css";

type WorkspacePageFrameProps = HTMLAttributes<HTMLDivElement>;

export function WorkspacePageFrame({ children, className, ...props }: Readonly<WorkspacePageFrameProps>) {
  const classes = [styles.frame, className].filter(Boolean).join(" ");

  return (
    <div className={classes} {...props}>
      <div className={styles.content}>{children}</div>
    </div>
  );
}
