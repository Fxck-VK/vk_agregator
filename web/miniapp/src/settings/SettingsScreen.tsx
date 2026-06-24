import { useCallback, useEffect, useMemo, useState } from "react";
import { Button } from "@vkontakte/vkui";
import bridge from "@vkontakte/vk-bridge";
import {
  MINIAPP_DARK_THEME_ONLY_ENABLED,
  MINIAPP_PAYMENT_CANCEL_ENABLED,
  apiUserMessage,
  cancelPaymentIntent,
  createIdempotencyKey,
  createPaymentIntent,
  getReferral,
  listPaymentIntents,
  listPaymentProducts,
  statusKind,
  statusLabel,
  type Job,
  type PaymentIntent,
  type PaymentProduct,
  type ReferralInfo,
} from "../api/client";
import { modalityByOperation, type ModalityId } from "../chat/types";
import neuroHubBanner from "../assets/neurohub-banner.png";
import { formatCredits } from "../ui/credits";
import { dedupeHistoryJobs, historyCountLabel, jobDisplayTitle } from "../utils/jobDisplay";
import { openExternalUrl, safeExternalHttpsUrl } from "../utils/openExternalUrl";
import type { ThemeMode } from "./theme";

type SettingsScreenProps = {
  themeMode: ThemeMode;
  balance: number | null;
  jobs: Job[];
  loading: boolean;
  onThemeModeChange: (mode: ThemeMode) => void;
  onClearLocalHistory: () => void;
  onRefreshBalance: () => void;
  onHistoryJobClick: (job: Job) => void;
};

type HistoryFilter = "all" | ModalityId | "chat";

const THEME_OPTIONS: Array<{ id: ThemeMode; label: string }> = [
  { id: "system", label: "Авто" },
  { id: "dark", label: "Тёмная" },
  { id: "light", label: "Светлая" },
];

const FILTER_OPTIONS: Array<{ id: HistoryFilter; label: string }> = [
  { id: "all", label: "Все" },
  { id: "image", label: "Картинки" },
  { id: "video", label: "Видео" },
  { id: "text", label: "Диалоги" },
];

function formatRub(kopecks: number): string {
  return new Intl.NumberFormat("ru-RU", {
    style: "currency",
    currency: "RUB",
    maximumFractionDigits: 0,
  }).format(kopecks / 100);
}

function receiptContactPayload(value: string): { receipt_email?: string; receipt_phone?: string } | null {
  const trimmed = value.trim();
  if (!trimmed) return null;
  if (trimmed.includes("@")) {
    return { receipt_email: trimmed };
  }
  const digits = trimmed.replace(/[^\d+]/g, "");
  if (digits.length >= 10) {
    return { receipt_phone: digits };
  }
  return null;
}

function dateLabel(value: string): string {
  const ts = Date.parse(value);
  if (!Number.isFinite(ts)) return "";
  return new Intl.DateTimeFormat("ru-RU", {
    day: "2-digit",
    month: "short",
    hour: "2-digit",
    minute: "2-digit",
  }).format(ts);
}

function historyLabel(job: Job): string {
  const kind = statusKind(job.status);
  if (kind === "done") return "Готово";
  if (kind === "failed") return "Ошибка";
  return statusLabel(job.status);
}

function historyRowMeta(job: Job): string {
  const modality = modalityByOperation(job.operation);
  const typeLabel = job.operation === "text_generate" ? "Чат" : modality.label;
  return `${typeLabel} · ${historyLabel(job)} · ${dateLabel(job.created_at)}`;
}

function typeColor(operation: string): string {
  const id = modalityByOperation(operation).id;
  if (id === "image") return "#a855f7";
  if (id === "video") return "#ec4899";
  return "#22d3ee";
}

function isActivePaymentIntent(intent: PaymentIntent): boolean {
  return intent.status === "waiting_for_user" && Boolean(paymentConfirmationUrl(intent));
}

function findActivePaymentIntent(intents: PaymentIntent[]): PaymentIntent | null {
  return intents.find(isActivePaymentIntent) ?? null;
}

function upsertPaymentIntent(items: PaymentIntent[], intent: PaymentIntent): PaymentIntent[] {
  return [intent, ...items.filter((item) => item.id !== intent.id)];
}

function paymentStatusTone(status: string): "done" | "failed" | "progress" | "refund" {
  if (status === "succeeded") return "done";
  if (status === "failed" || status === "canceled" || status === "expired") return "failed";
  if (status === "refunded" || status === "partially_refunded") return "refund";
  return "progress";
}

function paymentConfirmationUrl(intent: PaymentIntent | null | undefined): string | null {
  return safeExternalHttpsUrl(intent?.confirmation_url);
}

export function SettingsScreen({
  themeMode,
  balance,
  jobs,
  loading,
  onThemeModeChange,
  onClearLocalHistory,
  onRefreshBalance,
  onHistoryJobClick,
}: SettingsScreenProps) {
  const [historyFilter, setHistoryFilter] = useState<HistoryFilter>("all");
  const [historyOpen, setHistoryOpen] = useState(false);
  const [referralOpen, setReferralOpen] = useState(false);
  const [topUpNotice, setTopUpNotice] = useState("");
  const [receiptContact, setReceiptContact] = useState("");
  const [paymentProducts, setPaymentProducts] = useState<PaymentProduct[]>([]);
  const [paymentIntents, setPaymentIntents] = useState<PaymentIntent[]>([]);
  const [creatingNewPayment, setCreatingNewPayment] = useState(false);
  const [productsLoading, setProductsLoading] = useState(false);
  const [paymentPendingCode, setPaymentPendingCode] = useState("");
  const [cancellingPaymentID, setCancellingPaymentID] = useState("");
  const [referralInfo, setReferralInfo] = useState<ReferralInfo | null>(null);
  const [referralLoading, setReferralLoading] = useState(false);
  const [referralNotice, setReferralNotice] = useState("");
  const [referralCopied, setReferralCopied] = useState(false);
  const [spinning, setSpinning] = useState(false);
  const activePaymentIntent = useMemo(() => findActivePaymentIntent(paymentIntents), [paymentIntents]);
  const activePaymentConfirmationUrl = paymentConfirmationUrl(activePaymentIntent);
  const referralSummary = useMemo(() => {
    if (referralLoading && !referralInfo) return "Загружаем ссылку";
    if (!referralInfo) return "Ссылка пока недоступна";
    return `${referralInfo.invited_count} приглашённых · ${referralInfo.registered_count} регистраций`;
  }, [referralInfo, referralLoading]);
  const visibleJobs = useMemo(() => {
    const deduped = dedupeHistoryJobs(jobs);
    if (historyFilter === "all") return deduped;
    if (historyFilter === "text") {
      return deduped.filter((job) => job.operation === "text_generate");
    }
    return deduped.filter((job) => modalityByOperation(job.operation).id === historyFilter);
  }, [jobs, historyFilter]);

  const refreshReferral = useCallback(async () => {
    setReferralLoading(true);
    try {
      const referral = await getReferral();
      setReferralInfo(referral);
      setReferralNotice("");
    } catch (error) {
      setReferralNotice(apiUserMessage(error));
    } finally {
      setReferralLoading(false);
    }
  }, []);

  const refreshPaymentIntents = useCallback(async () => {
    try {
      const intents = await listPaymentIntents();
      setPaymentIntents(intents);
    } catch (error) {
      void error;
    }
  }, []);

  const handleRefresh = () => {
    if (spinning) return;
    setSpinning(true);
    onRefreshBalance();
    void refreshPaymentIntents();
    void refreshReferral();
    window.setTimeout(() => setSpinning(false), 900);
  };

  useEffect(() => {
    let cancelled = false;
    setProductsLoading(true);
    void Promise.allSettled([listPaymentProducts(), listPaymentIntents()])
      .then(([products, intents]) => {
        if (cancelled) return;
        if (products.status === "fulfilled") {
          setPaymentProducts(products.value);
        } else {
          setTopUpNotice("Не удалось загрузить тарифы. Попробуйте позже.");
        }
        if (intents.status === "fulfilled") {
          setPaymentIntents(intents.value);
        }
      })
      .finally(() => {
        if (!cancelled) {
          setProductsLoading(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    void refreshReferral();
  }, [refreshReferral]);

  async function copyReferralLink() {
    const link = referralInfo?.invite_url;
    if (!link) return;
    try {
      await bridge.send("VKWebAppCopyText", { text: link });
    } catch {
      try {
        await navigator.clipboard.writeText(link);
      } catch {
        setReferralNotice("Не удалось скопировать ссылку");
        return;
      }
    }
    setReferralCopied(true);
    setReferralNotice("Ссылка скопирована");
    window.setTimeout(() => setReferralCopied(false), 1400);
  }

  async function shareReferralLink() {
    const link = referralInfo?.invite_url;
    if (!link) return;
    try {
      await bridge.send("VKWebAppShare", { link });
      setReferralNotice("");
    } catch {
      await copyReferralLink();
    }
  }

  async function handleTopUp(product: PaymentProduct) {
    if (paymentPendingCode) return;
    if (activePaymentIntent && !creatingNewPayment) {
      setTopUpNotice("У вас уже есть незавершенный платеж. Продолжите оплату или создайте новый платеж.");
      return;
    }
    const contact = receiptContactPayload(receiptContact);
    if (!contact) {
      setTopUpNotice("Укажите email или телефон для чека.");
      return;
    }
    setTopUpNotice("");
    setPaymentPendingCode(product.code);
    try {
      const intent = await createPaymentIntent(
        { product_code: product.code, ...contact, force_new: creatingNewPayment },
        { idempotencyKey: createIdempotencyKey() },
      );
      setPaymentIntents((items) => upsertPaymentIntent(items, intent));
      const confirmationUrl = paymentConfirmationUrl(intent);
      if (intent.reused_active_payment && confirmationUrl) {
        setCreatingNewPayment(false);
        setTopUpNotice(intent.notice || "У вас уже есть незавершенный платеж. После оплаты баланс обновится автоматически.");
        void refreshPaymentIntents();
        return;
      }
      if (!confirmationUrl) {
        setTopUpNotice("Платеж создан, но ссылка на оплату пока недоступна. Попробуйте обновить страницу.");
        void refreshPaymentIntents();
        return;
      }
      void refreshPaymentIntents();
      const opened = await openExternalUrl(confirmationUrl);
      setTopUpNotice(
        opened
          ? "Оплата открыта во внешнем окне. После оплаты баланс обновится автоматически."
          : "Платеж создан. Нажмите «Продолжить оплату», чтобы открыть оплату во внешнем окне.",
      );
    } catch (error) {
      setTopUpNotice(apiUserMessage(error));
    } finally {
      setPaymentPendingCode("");
    }
  }

  async function handleCancelPayment(intent: PaymentIntent) {
    if (!MINIAPP_PAYMENT_CANCEL_ENABLED || !intent.id || cancellingPaymentID) return;
    setTopUpNotice("");
    setCancellingPaymentID(intent.id);
    try {
      const canceled = await cancelPaymentIntent(intent.id);
      setPaymentIntents((items) => upsertPaymentIntent(items, canceled));
      setCreatingNewPayment(false);
      setTopUpNotice(
        paymentStatusTone(canceled.status) === "failed"
          ? "Платеж отменен."
          : "Запрос отмены отправлен. Обновите историю через несколько секунд.",
      );
      void refreshPaymentIntents();
    } catch (error) {
      setTopUpNotice(apiUserMessage(error));
    } finally {
      setCancellingPaymentID("");
    }
  }

  function handleCreateNewPayment() {
    setCreatingNewPayment(true);
    setTopUpNotice("Создайте новый платеж. После оплаты баланс обновится автоматически.");
  }

  return (
    <main className="settings-screen nh-scroll">
      <div className="nh-hero nh-hero--profile nh-hero--banner-only" aria-hidden="true">
        <img className="nh-hero__img" src={neuroHubBanner} alt="" />
        <div className="nh-hero__fade" />
      </div>

      <div className="settings-hero-copy settings-hero-copy--banner">
        <h1 className="nh-page-title">НейроХаб</h1>
        <p className="nh-page-sub">инструменты для нового поколения</p>
      </div>

      {MINIAPP_DARK_THEME_ONLY_ENABLED ? null : (
        <section className="settings-card" aria-labelledby="settings-theme-title">
          <h2 id="settings-theme-title" style={{ margin: "0 0 14px", fontSize: "15px" }}>
            Тема оформления
          </h2>
          <div className="theme-segment" role="group" aria-label="Тема приложения">
            {THEME_OPTIONS.map((option) => (
              <button
                key={option.id}
                type="button"
                className={"theme-segment__btn" + (themeMode === option.id ? " is-active" : "")}
                onClick={() => onThemeModeChange(option.id)}
              >
                <span>{option.label}</span>
              </button>
            ))}
          </div>
        </section>
      )}

      <section className="settings-card" aria-labelledby="settings-balance-title">
        <h2 id="settings-balance-title" style={{ margin: "0 0 14px", fontSize: "15px" }}>
          Баланс
        </h2>
        <div className="settings-balance-hero">
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
            <div>
              <p style={{ margin: "0 0 4px", fontSize: "12px", color: "var(--fg-muted)" }}>
                Текущий баланс
              </p>
              <strong
                style={{
                  fontSize: "30px",
                  fontWeight: 800,
                  background: "var(--gradient-brand)",
                  WebkitBackgroundClip: "text",
                  backgroundClip: "text",
                  WebkitTextFillColor: "transparent",
                }}
              >
                {balance === null ? "..." : formatCredits(balance)}
              </strong>
            </div>
            <button
              type="button"
              className="chat__history-btn"
              aria-label="Обновить баланс"
              onClick={handleRefresh}
            >
              <svg
                width="16"
                height="16"
                viewBox="0 0 24 24"
                fill="none"
                className={spinning ? "nh-spin" : ""}
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
        </div>
        {activePaymentIntent && activePaymentConfirmationUrl && !creatingNewPayment ? (
          <div className="payment-pending" role="status">
            <strong>У вас есть незавершенный платеж</strong>
            <span>
              {formatCredits(activePaymentIntent.credits)} · {formatRub(activePaymentIntent.amount)}
            </span>
            <p>После оплаты баланс обновится автоматически.</p>
            <div className="payment-pending__actions">
              <a href={activePaymentConfirmationUrl} target="_blank" rel="noopener noreferrer">
                Продолжить оплату
              </a>
              <button type="button" onClick={handleCreateNewPayment}>
                Создать новый платеж
              </button>
              {MINIAPP_PAYMENT_CANCEL_ENABLED ? (
                <button
                  type="button"
                  disabled={cancellingPaymentID === activePaymentIntent.id}
                  onClick={() => void handleCancelPayment(activePaymentIntent)}
                >
                  {cancellingPaymentID === activePaymentIntent.id ? "Отменяем..." : "Отменить платеж"}
                </button>
              ) : null}
            </div>
          </div>
        ) : (
          <>
            <p className="settings-notice">После оплаты баланс обновится автоматически.</p>
            <div className="payment-contact">
              <label htmlFor="payment-contact-input">Email или телефон для чека</label>
              <input
                id="payment-contact-input"
                type="text"
                inputMode="email"
                autoComplete="email"
                placeholder="user@example.com"
                value={receiptContact}
                onChange={(event) => setReceiptContact(event.target.value)}
              />
            </div>
            {creatingNewPayment && activePaymentConfirmationUrl ? (
              <a
                className="payment-current-link"
                href={activePaymentConfirmationUrl}
                target="_blank"
                rel="noopener noreferrer"
              >
                Продолжить текущий платеж
              </a>
            ) : null}
            <div className="payment-products" aria-label="Тарифы пополнения">
              {productsLoading ? (
                <p className="settings-notice">Загружаем тарифы...</p>
              ) : paymentProducts.length === 0 ? (
                <p className="settings-notice">Тарифы пока недоступны.</p>
              ) : (
                paymentProducts.map((product) => (
                  <button
                    key={product.code}
                    type="button"
                    className="payment-product"
                    disabled={Boolean(paymentPendingCode)}
                    onClick={() => void handleTopUp(product)}
                  >
                    <span>
                      <strong>{formatCredits(product.credits)}</strong>
                      <small>{product.title}</small>
                    </span>
                    <em>{paymentPendingCode === product.code ? "Создаем..." : formatRub(product.amount)}</em>
                  </button>
                ))
              )}
            </div>
          </>
        )}
        {topUpNotice && <p className="settings-notice">{topUpNotice}</p>}
      </section>

      <section className="settings-card referral-card" aria-labelledby="settings-referral-title">
        <div className="referral-card__head">
          <button
            type="button"
            className={"referral-toggle" + (referralOpen ? " is-open" : "")}
            aria-expanded={referralOpen}
            aria-controls="settings-referral-panel"
            onClick={() => setReferralOpen((open) => !open)}
          >
            <span className="referral-toggle__main">
              <span id="settings-referral-title" className="referral-toggle__title">
                Реферальная программа
              </span>
              <span className="referral-toggle__subtitle">безлимитное общение с НейроХаб</span>
              <span className="referral-toggle__summary">{referralSummary}</span>
            </span>
            <svg
              className="referral-toggle__chevron"
              width="16"
              height="16"
              viewBox="0 0 24 24"
              fill="none"
              aria-hidden="true"
            >
              <path
                d="m6 9 6 6 6-6"
                stroke="currentColor"
                strokeWidth="2"
                strokeLinecap="round"
                strokeLinejoin="round"
              />
            </svg>
          </button>
          <button
            type="button"
            className="chat__history-btn"
            aria-label="Обновить реферальную статистику"
            onClick={() => void refreshReferral()}
          >
            <svg
              width="16"
              height="16"
              viewBox="0 0 24 24"
              fill="none"
              className={referralLoading ? "nh-spin" : ""}
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

        <div
          id="settings-referral-panel"
          className={"referral-panel" + (referralOpen ? " is-open" : "")}
          hidden={!referralOpen}
        >
          {referralLoading && !referralInfo ? (
            <div className="settings-empty">Загружаем ссылку</div>
          ) : referralInfo ? (
            <>
              <div className="referral-stats" aria-label="Реферальная статистика">
                <div>
                  <span>Приглашённых</span>
                  <strong>{referralInfo.invited_count}</strong>
                </div>
                <div>
                  <span>Зарегистрировано</span>
                  <strong>{referralInfo.registered_count}</strong>
                </div>
                <div>
                  <span>Активировано</span>
                  <strong>{referralInfo.activated_count}</strong>
                </div>
                <div>
                  <span>Бонус начислен</span>
                  <strong>{referralInfo.rewarded_count}</strong>
                </div>
                <div>
                  <span>Бонус другу</span>
                  <strong>+{formatCredits(referralInfo.referrer_signup_reward_credits)}</strong>
                </div>
              </div>
              <div className="referral-link-box">
                <span>{referralInfo.code}</span>
                <p>{referralInfo.invite_url || "Ссылка появится после настройки VK_REFERRAL_LINK_BASE"}</p>
              </div>
              <div className="referral-actions">
                <button type="button" disabled={!referralInfo.invite_url} onClick={() => void shareReferralLink()}>
                  Поделиться
                </button>
                <button type="button" disabled={!referralInfo.invite_url} onClick={() => void copyReferralLink()}>
                  {referralCopied ? "Скопировано" : "Скопировать"}
                </button>
              </div>
              <p className="settings-notice">Поддержка: @neirohub_help</p>
            </>
          ) : (
            <div className="settings-empty">Реферальная ссылка пока недоступна</div>
          )}
          {referralNotice && <p className="settings-notice">{referralNotice}</p>}
        </div>
      </section>

      <section className="settings-card settings-history-card" aria-labelledby="settings-history-title">
        <button
          type="button"
          className={"settings-history-toggle" + (historyOpen ? " is-open" : "")}
          aria-expanded={historyOpen}
          aria-controls="settings-history-panel"
          onClick={() => setHistoryOpen((open) => !open)}
        >
          <span className="settings-history-toggle__main">
            <span id="settings-history-title" className="settings-history-toggle__title">
              История запросов
            </span>
            <span className="settings-history-toggle__count">
              {loading ? "..." : historyCountLabel(visibleJobs.length)}
            </span>
          </span>
          <svg
            className="settings-history-toggle__chevron"
            width="16"
            height="16"
            viewBox="0 0 24 24"
            fill="none"
            aria-hidden="true"
          >
            <path
              d="m6 9 6 6 6-6"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
            />
          </svg>
        </button>

        <div
          id="settings-history-panel"
          className={"settings-history-panel" + (historyOpen ? " is-open" : "")}
          hidden={!historyOpen}
        >
          <div className="settings-filter-pills" role="group" aria-label="Фильтр истории">
            {FILTER_OPTIONS.map((option) => (
              <button
                key={option.id}
                type="button"
                className={"settings-filter-pill" + (historyFilter === option.id ? " is-active" : "")}
                onClick={() => setHistoryFilter(option.id)}
              >
                {option.label}
              </button>
            ))}
          </div>
          {loading ? (
            <div className="settings-empty">Загружаем историю генераций</div>
          ) : visibleJobs.length === 0 ? (
            <div className="settings-empty">Нет записей</div>
          ) : (
            <div className="settings-history-list">
              {visibleJobs.slice(0, 30).map((job) => {
                const modality = modalityByOperation(job.operation);
                const color = typeColor(job.operation);
                const backendJobCredits =
                  job.cost_captured > 0 ? job.cost_captured : job.cost_estimate;
                const canOpen =
                  job.operation === "text_generate" ||
                  job.operation === "image_generate" ||
                  job.operation === "video_generate";
                return (
                  <button
                    key={job.id}
                    type="button"
                    className="settings-history-row"
                    disabled={!canOpen}
                    onClick={() => onHistoryJobClick(job)}
                  >
                    <div
                      style={{
                        width: 36,
                        height: 36,
                        borderRadius: 10,
                        background: `${color}18`,
                        border: `1px solid ${color}35`,
                        display: "grid",
                        placeItems: "center",
                        color,
                        flexShrink: 0,
                      }}
                      aria-hidden="true"
                    >
                      <span style={{ fontSize: 11, fontWeight: 800 }}>{modality.label[0]}</span>
                    </div>
                    <div style={{ flex: 1, minWidth: 0 }}>
                      <div style={{ fontSize: 14, fontWeight: 600 }}>
                        {jobDisplayTitle(job)}
                      </div>
                      <div style={{ fontSize: 12, color: "var(--fg-muted)" }}>
                        {historyRowMeta(job)}
                        {backendJobCredits > 0 ? ` · ${formatCredits(backendJobCredits)}` : ""}
                      </div>
                    </div>
                    <svg width="15" height="15" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                      <path
                        d="m9 6 6 6-6 6"
                        stroke="var(--fg-muted)"
                        strokeWidth="2"
                        strokeLinecap="round"
                        strokeLinejoin="round"
                      />
                    </svg>
                  </button>
                );
              })}
            </div>
          )}
        </div>
      </section>

      <section className="settings-card" aria-labelledby="settings-privacy-title">
        <h2 id="settings-privacy-title" style={{ margin: "0 0 12px", fontSize: "15px" }}>
          Локальные данные
        </h2>
        <p style={{ margin: "0 0 12px", color: "var(--fg-muted)", fontSize: 14, lineHeight: 1.45 }}>
          На устройстве хранятся только UI-настройки: активная вкладка, тема и метаданные диалогов.
        </p>
        <Button
          type="button"
          mode="secondary"
          appearance="neutral"
          size="l"
          stretched
          onClick={onClearLocalHistory}
        >
          Очистить локальную историю
        </Button>
      </section>
    </main>
  );
}
