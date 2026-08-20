import type { CSSProperties, HTMLAttributes } from "react";

import styles from "./AssetIcon.module.css";

export type AssetIconProps = Omit<HTMLAttributes<HTMLSpanElement>, "children"> & {
  iconName: string;
  source: string;
};

type AssetIconStyle = CSSProperties & {
  "--asset-icon-source": string;
};

export function AssetIcon({ className, iconName, source, style, ...props }: Readonly<AssetIconProps>) {
  const iconStyle: AssetIconStyle = {
    ...style,
    "--asset-icon-source": `url("${source}")`,
  };

  return (
    <span
      {...props}
      aria-hidden="true"
      className={[styles.icon, className].filter(Boolean).join(" ")}
      data-icon={iconName}
      style={iconStyle}
    />
  );
}
