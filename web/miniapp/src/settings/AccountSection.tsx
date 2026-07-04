import { useCallback, useEffect, useMemo, useState } from "react";

import {
  apiUserMessage,
  getAccountProfile,
  requestAccountEmailCode,
  requestAccountPhoneOTP,
  unlinkAccountIdentity,
  verifyAccountEmailCode,
  verifyAccountPhoneOTP,
  type AccountIdentity,
  type AccountProfile,
} from "../api/client";

type LinkMode = "email" | "phone" | null;
type CodeStep = "input" | "code";

function shortAccountID(id: string): string {
  if (!id) return "...";
  if (id.length <= 14) return id;
  return `${id.slice(0, 8)}...${id.slice(-4)}`;
}

function providerLabel(provider: string): string {
  switch (provider) {
    case "vk":
      return "VK";
    case "telegram":
      return "Telegram";
    case "google":
      return "Google";
    case "apple":
      return "Apple";
    case "email":
      return "Email";
    case "phone":
      return "Телефон";
    case "password":
      return "Пароль";
    default:
      return "Способ входа";
  }
}

function ttlLabel(seconds: number): string {
  if (seconds >= 60) {
    return `${Math.ceil(seconds / 60)} мин`;
  }
  return `${Math.max(seconds, 0)} сек`;
}

function canUnlink(identity: AccountIdentity, total: number): boolean {
  return identity.provider !== "vk" && total > 1;
}

function identityMeta(identity: AccountIdentity): string {
  return identity.verified ? "Подтверждено" : "Не подтверждено";
}

export function AccountSection() {
  const [profile, setProfile] = useState<AccountProfile | null>(null);
  const [loading, setLoading] = useState(false);
  const [notice, setNotice] = useState("");
  const [pending, setPending] = useState("");
  const [linkMode, setLinkMode] = useState<LinkMode>(null);
  const [email, setEmail] = useState("");
  const [emailCode, setEmailCode] = useState("");
  const [emailStep, setEmailStep] = useState<CodeStep>("input");
  const [phone, setPhone] = useState("");
  const [phoneCode, setPhoneCode] = useState("");
  const [phoneStep, setPhoneStep] = useState<CodeStep>("input");
  const [confirmUnlinkID, setConfirmUnlinkID] = useState("");

  const identities = useMemo(() => profile?.identity_refs ?? [], [profile]);

  const refreshAccount = useCallback(async () => {
    setLoading(true);
    try {
      const data = await getAccountProfile();
      setProfile(data);
      setNotice("");
    } catch (error) {
      setNotice(apiUserMessage(error));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void refreshAccount();
  }, [refreshAccount]);

  function openLinkMode(mode: Exclude<LinkMode, null>) {
    setLinkMode(mode);
    setNotice("");
    setConfirmUnlinkID("");
    if (mode === "email") {
      setEmailStep("input");
      setEmailCode("");
    } else {
      setPhoneStep("input");
      setPhoneCode("");
    }
  }

  async function handleEmailSubmit() {
    const trimmed = email.trim();
    if (!trimmed || pending) return;
    setPending("email");
    setNotice("");
    try {
      if (emailStep === "input") {
        const result = await requestAccountEmailCode(trimmed);
        setEmailStep("code");
        setNotice(`Код отправлен. Действует ${ttlLabel(result.expires_in_seconds)}`);
        return;
      }
      await verifyAccountEmailCode(trimmed, emailCode.trim());
      setEmail("");
      setEmailCode("");
      setEmailStep("input");
      setLinkMode(null);
      setNotice("Email привязан");
      await refreshAccount();
    } catch (error) {
      setNotice(apiUserMessage(error));
    } finally {
      setPending("");
    }
  }

  async function handlePhoneSubmit() {
    const trimmed = phone.trim();
    if (!trimmed || pending) return;
    setPending("phone");
    setNotice("");
    try {
      if (phoneStep === "input") {
        const result = await requestAccountPhoneOTP(trimmed);
        setPhoneStep("code");
        setNotice(`Код отправлен. Действует ${ttlLabel(result.expires_in_seconds)}`);
        return;
      }
      await verifyAccountPhoneOTP(trimmed, phoneCode.trim());
      setPhone("");
      setPhoneCode("");
      setPhoneStep("input");
      setLinkMode(null);
      setNotice("Телефон привязан");
      await refreshAccount();
    } catch (error) {
      setNotice(apiUserMessage(error));
    } finally {
      setPending("");
    }
  }

  async function handleUnlink(identity: AccountIdentity) {
    if (!canUnlink(identity, identities.length) || pending) return;
    if (confirmUnlinkID !== identity.id) {
      setConfirmUnlinkID(identity.id);
      setNotice("Нажмите еще раз, чтобы отвязать способ входа");
      return;
    }
    setPending(identity.id);
    setNotice("");
    try {
      await unlinkAccountIdentity(identity.id);
      setConfirmUnlinkID("");
      setNotice("Способ входа отвязан");
      await refreshAccount();
    } catch (error) {
      setNotice(apiUserMessage(error));
    } finally {
      setPending("");
    }
  }

  return (
    <section className="settings-card account-card" aria-labelledby="settings-account-title">
      <div className="account-card__head">
        <div>
          <h2 id="settings-account-title">Аккаунт</h2>
          <p>Единый профиль НейроХаб</p>
        </div>
        <button
          type="button"
          className="chat__history-btn"
          aria-label="Обновить аккаунт"
          onClick={() => void refreshAccount()}
        >
          <svg
            width="16"
            height="16"
            viewBox="0 0 24 24"
            fill="none"
            className={loading ? "nh-spin" : ""}
            aria-hidden="true"
          >
            <path
              d="M3 12a9 9 0 1 0 3-6.7M3 3v6h6"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
            />
          </svg>
        </button>
      </div>

      {profile ? (
        <div className="account-summary">
          <span>ID аккаунта</span>
          <strong>{shortAccountID(profile.account_id)}</strong>
          <small>{identities.length} способов входа</small>
        </div>
      ) : loading ? (
        <div className="settings-empty">Загружаем аккаунт</div>
      ) : null}

      <div className="account-identities">
        <div className="account-section-title">Способы входа</div>
        {identities.length === 0 && !loading ? (
          <div className="settings-empty">Способы входа пока недоступны</div>
        ) : (
          identities.map((identity) => (
            <div key={identity.id} className="account-identity">
              <div className="account-identity__main">
                <div className="account-identity__title">
                  <strong>{providerLabel(identity.provider)}</strong>
                  <span className="account-identity__badge">{identityMeta(identity)}</span>
                </div>
                <span className="account-identity__label">{identity.label || providerLabel(identity.provider)}</span>
              </div>
              {canUnlink(identity, identities.length) ? (
                <button
                  type="button"
                  className="account-identity__action"
                  disabled={pending === identity.id}
                  onClick={() => void handleUnlink(identity)}
                >
                  {confirmUnlinkID === identity.id ? "Подтвердить" : "Отвязать"}
                </button>
              ) : null}
            </div>
          ))
        )}
      </div>

      <div className="account-link-actions">
        <button type="button" onClick={() => openLinkMode("email")}>
          Привязать email
        </button>
        <button type="button" onClick={() => openLinkMode("phone")}>
          Привязать телефон
        </button>
      </div>

      {linkMode === "email" ? (
        <div className="account-link-form">
          <label htmlFor="account-email-input">Email</label>
          <input
            id="account-email-input"
            type="email"
            autoComplete="email"
            placeholder="user@example.com"
            value={email}
            onChange={(event) => setEmail(event.target.value)}
            disabled={emailStep === "code"}
          />
          {emailStep === "code" ? (
            <>
              <label htmlFor="account-email-code-input">Код</label>
              <input
                id="account-email-code-input"
                type="text"
                inputMode="numeric"
                autoComplete="one-time-code"
                value={emailCode}
                onChange={(event) => setEmailCode(event.target.value)}
              />
            </>
          ) : null}
          <div className="account-link-form__actions">
            <button type="button" disabled={pending === "email"} onClick={() => void handleEmailSubmit()}>
              {emailStep === "input" ? "Получить код" : "Привязать"}
            </button>
            <button type="button" onClick={() => setLinkMode(null)}>
              Отмена
            </button>
          </div>
        </div>
      ) : null}

      {linkMode === "phone" ? (
        <div className="account-link-form">
          <label htmlFor="account-phone-input">Телефон</label>
          <input
            id="account-phone-input"
            type="tel"
            autoComplete="tel"
            placeholder="+79990000000"
            value={phone}
            onChange={(event) => setPhone(event.target.value)}
            disabled={phoneStep === "code"}
          />
          {phoneStep === "code" ? (
            <>
              <label htmlFor="account-phone-code-input">Код</label>
              <input
                id="account-phone-code-input"
                type="text"
                inputMode="numeric"
                autoComplete="one-time-code"
                value={phoneCode}
                onChange={(event) => setPhoneCode(event.target.value)}
              />
            </>
          ) : null}
          <div className="account-link-form__actions">
            <button type="button" disabled={pending === "phone"} onClick={() => void handlePhoneSubmit()}>
              {phoneStep === "input" ? "Получить код" : "Привязать"}
            </button>
            <button type="button" onClick={() => setLinkMode(null)}>
              Отмена
            </button>
          </div>
        </div>
      ) : null}

      {notice ? <p className="settings-notice">{notice}</p> : null}
    </section>
  );
}
