import { forwardRef, type ButtonHTMLAttributes } from "react";

import styles from "./Button.module.css";

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement>;

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(function Button(
  { className, type = "button", ...props },
  ref,
) {
  const classes = [styles.button, className].filter(Boolean).join(" ");

  return <button className={classes} ref={ref} type={type} {...props} />;
});
