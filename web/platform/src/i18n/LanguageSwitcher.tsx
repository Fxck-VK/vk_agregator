"use client";

import { StateNotice } from "@/components/ui/AsyncState/AsyncState";
import { useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import { ModeSwitchPanel } from "@/components/ui/ModeSwitchPanel/ModeSwitchPanel";
import { setLocalePreference } from "./actions";
import { useDictionary, useLocale } from "./LocaleProvider";
import { locales, localeNames, type Locale } from "./locales";
import { localizeHref } from "./routing";

export function LanguageSwitcher() {
  const locale = useLocale();
  const router = useRouter();
  const t = useDictionary();
  const [isPending, startTransition] = useTransition();
  const [hasError, setHasError] = useState(false);
  const changeLocale = (next: Locale) => {
    if (next === locale) return;
    setHasError(false);
    startTransition(async () => {
      try {
        await setLocalePreference(next);
        const { pathname, search, hash } = window.location;
        router.push(localizeHref(`${pathname}${search}${hash}`, next), { scroll: false });
      }
      catch { setHasError(true); }
    });
  };
  return (
    <div>
      <ModeSwitchPanel
        activeID={locale}
        ariaLabel={t.account.languageLabel}
        items={locales.map(id => ({ id, label: localeNames[id], disabled: isPending }))}
        onChange={changeLocale}
      />
      {hasError ? <StateNotice inline kind="error">{t.account.languageFailure}</StateNotice> : null}
    </div>
  );
}
