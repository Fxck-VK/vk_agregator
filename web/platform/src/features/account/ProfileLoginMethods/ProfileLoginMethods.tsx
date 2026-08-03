import type { AccountProfile } from "@/lib/web-api/contracts";
import { ru } from "@/i18n/ru";

import styles from "./ProfileLoginMethods.module.css";

type ProfileLoginMethodsProps = {
  identityRefs: AccountProfile["identity_refs"];
};

function getProviderName(provider: string): string {
  switch (provider.trim().toLocaleLowerCase()) {
    case "email":
    case "web_email":
      return ru.profile.emailProviderLabel;
    case "vk":
      return "VK";
    case "telegram":
      return "Telegram";
    case "yandex":
      return "Яндекс";
    default:
      return ru.profile.genericProviderLabel;
  }
}

function getProviderGlyph(provider: string): string {
  switch (provider.trim().toLocaleLowerCase()) {
    case "email":
    case "web_email":
      return "@";
    case "vk":
      return "VK";
    case "telegram":
      return "↗";
    case "yandex":
      return "Я";
    default:
      return "•";
  }
}

export function ProfileLoginMethods({ identityRefs }: ProfileLoginMethodsProps) {
  const visibleRefs = identityRefs.filter((identity) => identity.verified);

  return (
    <section aria-labelledby="profile-login-methods-title" className={styles.section}>
      <h2 id="profile-login-methods-title">{ru.profile.loginMethodsTitle}</h2>
      <div className={styles.card}>
        {visibleRefs.length > 0 ? (
          <ul className={styles.list}>
            {visibleRefs.map((identity) => (
              <li key={identity.id}>
                <span aria-hidden="true" className={styles.glyph}>{getProviderGlyph(identity.provider)}</span>
                <span className={styles.details}>
                  <strong>{getProviderName(identity.provider)}</strong>
                  <span>{identity.label}</span>
                </span>
              </li>
            ))}
          </ul>
        ) : <p>{ru.profile.noLoginMethods}</p>}
      </div>
    </section>
  );
}
