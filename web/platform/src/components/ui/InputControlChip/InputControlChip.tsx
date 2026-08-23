import type { ComponentPropsWithRef } from "react";

import styles from "./InputControlChip.module.css";

type ButtonChipProps = ComponentPropsWithRef<"button"> & {
  as?: "button";
};

type GroupChipProps = ComponentPropsWithRef<"div"> & {
  as: "div";
};

type InputControlChipProps = ButtonChipProps | GroupChipProps;

export function InputControlChip(props: Readonly<InputControlChipProps>) {
  const { as, className, ...elementProps } = props;

  if (as === "div") {
    const groupProps = elementProps as ComponentPropsWithRef<"div">;
    const classes = [styles.chip, styles.group, className].filter(Boolean).join(" ");

    return (
      <div
        {...groupProps}
        className={classes}
        data-ui="input-control-chip"
      />
    );
  }

  const { type = "button", ...buttonProps } = elementProps as ComponentPropsWithRef<"button">;
  const classes = [styles.chip, styles.button, className].filter(Boolean).join(" ");

  return (
    <button
      {...buttonProps}
      className={classes}
      data-ui="input-control-chip"
      type={type}
    />
  );
}
