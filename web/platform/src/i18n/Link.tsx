"use client";

import NextLink from "next/link";
import type { ComponentProps } from "react";
import { useLocale } from "./LocaleProvider";
import { localizeHref } from "./routing";

export type { LinkProps } from "next/link";

export default function Link({ href, ...props }: ComponentProps<typeof NextLink>) {
  const locale = useLocale();
  const localized = typeof href === "string"
    ? localizeHref(href, locale)
    : href.host || href.hostname || href.protocol || !href.pathname
      ? href
      : { ...href, pathname: localizeHref(href.pathname, locale) };
  return <NextLink {...props} href={localized} />;
}
