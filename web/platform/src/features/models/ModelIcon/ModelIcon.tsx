import Image from "next/image";

import { assetPaths } from "@/assets/asset-paths";

import styles from "./ModelIcon.module.css";

type ModelIconProps = {
  className?: string;
  src?: string | null;
};

export function ModelIcon({ className, src }: Readonly<ModelIconProps>) {
  const classNames = [styles.icon, className].filter(Boolean).join(" ");

  return (
    <Image
      alt=""
      aria-hidden="true"
      className={classNames}
      data-testid="model-icon"
      height={245}
      src={src || assetPaths.images.models.fallback}
      width={205}
    />
  );
}
