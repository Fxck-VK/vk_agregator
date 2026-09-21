import { forwardRef, type ButtonHTMLAttributes } from "react";

import styles from "./Button.module.css";

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: "filled" | "outline";
};

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(function Button(
  { className, type = "button", variant = "filled", ...props },
  ref,
) {
  const classes = [styles.button, variant === "outline" && styles.outline, className].filter(Boolean).join(" ");

  return <button className={classes} ref={ref} type={type} {...props} />;
});
