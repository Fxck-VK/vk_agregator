"use client";

import { useMessages, useDictionary } from "@/i18n/LocaleProvider";

import { getTranslator, type Translator } from "@/i18n/messages";

import type { AccountProfile } from "@/lib/web-api/contracts";
import type { Dictionary } from "@/i18n/dictionary";

import styles from "./ProfileLoginMethods.module.css";

type ProfileLoginMethodsProps = {
  identityRefs: AccountProfile["identity_refs"];
};

function getProviderName(provider: string, t: Dictionary, msg: Translator = getTranslator("ru")): string {
  switch (provider.trim().toLocaleLowerCase()) {
    case "email":
    case "web_email":
      return t.profile.emailProviderLabel;
    case "vk":
      return "VK";
    case "telegram":
      return "Telegram";
    case "yandex":
      return msg("profileLoginMethods.yandex");
    default:
      return t.profile.genericProviderLabel;
  }
}

function getProviderGlyph(provider: string, msg: Translator = getTranslator("ru")): string {
  switch (provider.trim().toLocaleLowerCase()) {
    case "email":
    case "web_email":
      return "@";
    case "vk":
      return "VK";
    case "telegram":
      return "↗";
    case "yandex":
      return msg("profileLoginMethods.y");
    default:
      return "•";
  }
}

export function ProfileLoginMethods({ identityRefs }: ProfileLoginMethodsProps) {
  const msg = useMessages();
  const t = useDictionary();
  const visibleRefs = identityRefs.filter((identity) => identity.verified);

  return (
    <section aria-labelledby="profile-login-methods-title" className={styles.section}>
      <h2 id="profile-login-methods-title">{t.profile.loginMethodsTitle}</h2>
      <div className={styles.card}>
        {visibleRefs.length > 0 ? (
          <ul className={styles.list}>
            {visibleRefs.map((identity) => (
              <li key={identity.id}>
                <span aria-hidden="true" className={styles.glyph}>{getProviderGlyph(identity.provider, msg)}</span>
                <span className={styles.details}>
                  <strong>{getProviderName(identity.provider, t, msg)}</strong>
                  <span>{identity.label}</span>
                </span>
              </li>
            ))}
          </ul>
        ) : <p>{t.profile.noLoginMethods}</p>}
      </div>
    </section>
  );
}
