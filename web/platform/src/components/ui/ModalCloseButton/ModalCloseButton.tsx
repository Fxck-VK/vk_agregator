import { forwardRef, type ButtonHTMLAttributes } from "react";

import styles from "./ModalCloseButton.module.css";

type ModalCloseButtonProps = ButtonHTMLAttributes<HTMLButtonElement>;

export const ModalCloseButton = forwardRef<HTMLButtonElement, ModalCloseButtonProps>(
  function ModalCloseButton({ className, type = "button", ...props }, ref) {
    const classes = [styles.button, className].filter(Boolean).join(" ");

    return (
      <button className={classes} ref={ref} type={type} {...props}>
        <span aria-hidden="true" className={styles.icon} />
      </button>
    );
  },
);
