"use client";

import NextLink from "next/link";
import type { ComponentProps } from "react";
import { useLocale } from "./LocaleProvider";
import { localizeHref } from "./routing";
import { useReleaseView } from "@/features/release/ReleaseViewProvider";

export type { LinkProps } from "next/link";

export default function Link({ href, ...props }: ComponentProps<typeof NextLink>) {
  const locale = useLocale();
  const release = useReleaseView();
  const destination = typeof href === "string" ? href : href.host || href.hostname || href.protocol ? null : href.pathname;
  if (destination && destination !== "/" && !release.allowsPath(destination)) return null;
  const localized = typeof href === "string"
    ? localizeHref(href, locale)
    : href.host || href.hostname || href.protocol || !href.pathname
      ? href
      : { ...href, pathname: localizeHref(href.pathname, locale) };
  return <NextLink {...props} href={localized} />;
}
