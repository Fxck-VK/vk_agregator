import { forwardRef, type ButtonHTMLAttributes } from "react";

import styles from "./ModalCloseButton.module.css";

type ModalCloseButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  size?: "default" | "compact";
};

export const ModalCloseButton = forwardRef<HTMLButtonElement, ModalCloseButtonProps>(
  function ModalCloseButton({ className, size = "default", type = "button", ...props }, ref) {
    const classes = [styles.button, size === "compact" && styles.compact, className].filter(Boolean).join(" ");

    return (
      <button className={classes} ref={ref} type={type} {...props}>
        <span aria-hidden="true" className={styles.icon} />
      </button>
    );
  },
);
