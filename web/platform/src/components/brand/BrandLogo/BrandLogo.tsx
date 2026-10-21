import Image from "next/image";

import { brandLogoPath } from "@/config/brand";
import styles from "./BrandLogo.module.css";

export function BrandLogo({ className, size = 40, testId }: { className?: string; size?: number; testId?: string }) {
  return (
    <Image
      alt=""
      aria-hidden="true"
      className={[styles.logo, className].filter(Boolean).join(" ")}
      data-brand-logo
      data-testid={testId}
      height={size}
      src={brandLogoPath}
      width={size}
    />
  );
}
