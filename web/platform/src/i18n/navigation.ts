"use client";

import { usePathname as useNextPathname, useRouter as useNextRouter } from "next/navigation";
import { useMemo } from "react";
import { useLocale } from "./LocaleProvider";
import { localizeHref, stripLocale } from "./routing";

export { notFound, useSearchParams } from "next/navigation";

/** Domain components compare page paths independently of their display language. */
export function usePathname() {
  const pathname = useNextPathname();
  return pathname ? stripLocale(pathname) : pathname;
}

export function useRouter() {
  const router = useNextRouter();
  const locale = useLocale();
  return useMemo(() => ({
    ...router,
    push: (href: string, options?: Parameters<typeof router.push>[1]) => options === undefined ? router.push(localizeHref(href, locale)) : router.push(localizeHref(href, locale), options),
    replace: (href: string, options?: Parameters<typeof router.replace>[1]) => options === undefined ? router.replace(localizeHref(href, locale)) : router.replace(localizeHref(href, locale), options),
    prefetch: (href: string, options?: Parameters<typeof router.prefetch>[1]) => options === undefined ? router.prefetch(localizeHref(href, locale)) : router.prefetch(localizeHref(href, locale), options),
  }), [router, locale]);
}
