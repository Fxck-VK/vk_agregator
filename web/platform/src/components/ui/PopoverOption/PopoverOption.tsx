import type { ComponentPropsWithRef } from "react";

import selectableStyles from "@/components/ui/selectable-control.module.css";

import styles from "./PopoverOption.module.css";

type PopoverOptionProps = Omit<ComponentPropsWithRef<"button">, "aria-checked" | "role"> & {
  selected: boolean;
};

export function PopoverOption({
  className,
  selected,
  type = "button",
  ...props
}: Readonly<PopoverOptionProps>) {
  return (
    <button
      {...props}
      aria-checked={selected}
      className={[selectableStyles.control, styles.option, className].filter(Boolean).join(" ")}
      data-ui="popover-option"
      role="radio"
      type={type}
    />
  );
}
