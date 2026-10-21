import { Fragment } from "react";

import { brandMonogram, brandName, brandWordmark } from "@/config/brand";

import styles from "./BrandName.module.css";

export function BrandName({ className, monogram = false }: { className?: string; monogram?: boolean }) {
  const lettering = !monogram && brandWordmark?.name === brandName ? brandWordmark : null;
  return (
    <span className={[styles.name, className].filter(Boolean).join(" ")} data-brand-name>
      {lettering ? <>
        <svg
          aria-hidden="true"
          className={styles.wordmark}
          data-brand-wordmark
          focusable="false"
          height={lettering.height}
          viewBox={`0 0 ${lettering.width} ${lettering.height}`}
          width={lettering.width}
        >
          <use href={`${lettering.path}#wordmark`} />
        </svg>
        <span className={styles.accessibleName}>{brandName}</span>
      </> : monogram ? brandMonogram : brandName}
    </span>
  );
}

/** Only for interface/editorial copy. Keep messages, prompts and file names unchanged. */
export function BrandText({ children }: { children: string }) {
  if (!brandName || !children.includes(brandName)) return children;
  const parts = [];
  const isWordCharacter = (value: string | undefined) => value !== undefined && /[\p{L}\p{N}_]/u.test(value);
  let start = 0;
  let searchFrom = 0;
  for (let index = children.indexOf(brandName, searchFrom); index !== -1; index = children.indexOf(brandName, searchFrom)) {
    const end = index + brandName.length;
    searchFrom = end;
    if (isWordCharacter(children[index - 1]) || isWordCharacter(children[end])) continue;
    parts.push(<Fragment key={index}>{children.slice(start, index)}<BrandName /></Fragment>);
    start = end;
  }
  return <>{parts}{children.slice(start)}</>;
}

// Native placeholders cannot contain a span; style only the placeholder, never the entered text.
export const brandPlaceholderClassName = styles.placeholder;
