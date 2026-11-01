import { useEffect, useRef, useState, type FormEvent } from "react";
import type { AdminClient } from "../api/adminClient";
import { toSafeAdminError } from "../api/adminClient";
import {
  buildPromotionConfig, fetchPromotions, preparePromotionMutation, promotionPlans, savePromotion,
  type Promotion, type PromotionAccess, type PromotionConfig, type PromotionDraft, type PromotionMutation,
} from "../api/promotions";
import { OperatorConfirmDialog } from "../components/OperatorConfirmDialog";

export function PromotionsScreen({ client, adminTokenSet, access }: { client: AdminClient; adminTokenSet: boolean; access: PromotionAccess }) {
  const [items, setItems] = useState<Promotion[]>([]);
  const [offset, setOffset] = useState(0);
  const [hasMore, setHasMore] = useState(false);
  const [loading, setLoading] = useState(true);
  const [reload, setReload] = useState(0);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");
  const [editing, setEditing] = useState<{ existing?: Promotion }>();
  const [pending, setPending] = useState<PromotionMutation>();
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [busy, setBusy] = useState(false);
  const [mutationError, setMutationError] = useState("");
  const inFlight = useRef(false);
  const refreshRequired = useRef(false);

  useEffect(() => {
    if (!adminTokenSet || !access.read) return;
    const controller = new AbortController();
    setLoading(true);
    setError("");
    fetchPromotions(client, offset, controller.signal).then((page) => {
      if (controller.signal.aborted) return;
      setItems(page.items);
      setHasMore(page.has_more);
      setLoading(false);
      refreshRequired.current = false;
    }).catch((reason: unknown) => {
      if (controller.signal.aborted) return;
      setItems([]);
      setHasMore(false);
      setError(toSafeAdminError(reason).message);
      setLoading(false);
    });
    return () => controller.abort();
  }, [client, adminTokenSet, access.read, offset, reload]);

  if (!adminTokenSet || !access.read) {
    return <article className="surface panel" role="status"><h3>Нет доступа к промокодам</h3><p>Нужно разрешение на просмотр промокодов.</p></article>;
  }

  function prepare(body: PromotionConfig) {
    if (!access.manage || pending || refreshRequired.current) return;
    setPending(preparePromotionMutation(body, editing?.existing?.id));
    setMutationError("");
    setConfirmOpen(true);
  }

  async function confirm() {
    if (!pending || !access.manage || inFlight.current) return;
    inFlight.current = true;
    setBusy(true);
    setMutationError("");
    try {
      await savePromotion(client, pending);
      setPending(undefined);
      setConfirmOpen(false);
      setEditing(undefined);
      refreshRequired.current = true;
      setMessage("Промокод сохранён. Список обновляется.");
      setReload((value) => value + 1);
    } catch (reason: unknown) {
      const safe = toSafeAdminError(reason);
      const safeMessage = promotionMutationError(reason);
      if (safe.status === 400 || safe.status === 401 || safe.status === 403 || safe.status === 404 || safe.status === 409) {
        setPending(undefined);
        setConfirmOpen(false);
        setEditing(undefined);
        setError(safeMessage);
        setMessage("");
        // A stale revision must never be overwritten with a fresh key from the old form.
        refreshRequired.current = true;
      } else {
        // Keep the original immutable body/key after timeouts and ambiguous responses.
        setMutationError(safeMessage);
      }
    } finally {
      inFlight.current = false;
      setBusy(false);
    }
  }

  return <div className="ops-stack promotions-console">
    <section className="surface queue-panel" aria-label="Управление промокодами">
      <div className="section-heading"><div><p className="eyebrow">Первый тариф</p><h3>Промокоды</h3></div><span>{loading ? "Загрузка" : `${items.length} на странице`}</span></div>
      <p className="muted">Скидка действует только на первую успешную покупку тарифа. Возврат не восстанавливает право на промокод и не освобождает использованное применение. Пополнения и продления оплачиваются без скидки.</p>
      <div className="retention-actions">
        <button type="button" disabled={loading || busy || Boolean(pending)} onClick={() => { setEditing(undefined); setReload((value) => value + 1); }}>Обновить список</button>
        {access.manage ? <button type="button" disabled={loading || busy || Boolean(pending) || refreshRequired.current} onClick={() => { setMessage(""); setEditing({}); }}>Создать промокод</button> : <p className="muted">Доступен только просмотр.</p>}
        {pending ? <button type="button" disabled={busy} onClick={() => setConfirmOpen(true)}>Вернуться к подтверждению</button> : null}
      </div>
      {error ? <p role="alert">{error}</p> : null}
      {message ? <p className="retention-message" role="status">{message}</p> : null}
    </section>
    {editing && access.manage ? <PromotionEditor key={editing.existing?.id ?? "new"} existing={editing.existing} locked={busy || Boolean(pending)} onPrepare={prepare} onCancel={() => setEditing(undefined)} /> : null}
    <PromotionList items={items} canManage={access.manage && !refreshRequired.current} busy={loading || busy || Boolean(pending)} offset={offset} hasMore={hasMore} onEdit={(existing) => { setMessage(""); setEditing({ existing }); }} onPage={(next) => { setEditing(undefined); setOffset(next); }} />
    {confirmOpen && pending && access.manage ? <OperatorConfirmDialog
      busy={busy} cancelLabel="Назад" confirmLabel={mutationError ? "Повторить сохранение" : "Сохранить промокод"}
      description="Подтвердите условия. Изменения сохраняются в журнале действий. Уже принятые платежи сохраняют прежние условия."
      eyebrow="Подтверждение оператора" title={pending.body.revision ? "Изменить промокод" : "Создать промокод"}
      onCancel={() => { if (!mutationError) setPending(undefined); setConfirmOpen(false); }} onConfirm={() => void confirm()}>
      <dl className="detail-list">
        <div><dt>Код</dt><dd>{pending.body.code}</dd></div>
        <div><dt>Скидка</dt><dd>{pending.body.percent}%</dd></div>
        <div><dt>Состояние</dt><dd>{pending.body.enabled ? "Включён" : "Выключен"}</dd></div>
        <div><dt>Начало</dt><dd>{pending.body.starts_at}</dd></div>
        <div><dt>Окончание</dt><dd>{pending.body.ends_at ?? "Без окончания"}</dd></div>
        <div><dt>Тарифы</dt><dd>{pending.body.plan_codes.join(", ")}</dd></div>
        <div><dt>Лимит</dt><dd>{pending.body.max_uses || "Без ограничений"}</dd></div>
      </dl>
      {mutationError ? <p role="alert">{mutationError}</p> : null}
    </OperatorConfirmDialog> : null}
  </div>;
}

export function PromotionList({ items, canManage, busy, offset, hasMore, onEdit, onPage }: { items: Promotion[]; canManage: boolean; busy: boolean; offset: number; hasMore: boolean; onEdit: (item: Promotion) => void; onPage: (offset: number) => void }) {
  return <section className="surface jobs-list" aria-label="Список промокодов">
    <h3>Условия и применения</h3>
    {items.length ? <div className="promotions-table" tabIndex={0} role="region" aria-label="Таблица промокодов, прокрутка по горизонтали">
      <div className="promotions-row promotions-row--head"><span>Код / версия</span><span>Скидка / статус</span><span>Период (UTC)</span><span>Тарифы</span><span>Применения</span><span>Действия</span></div>
      {items.map((item) => <div className="promotions-row" key={item.id}>
        <span className="promotion-code"><strong>{item.code}</strong><small>Версия {item.revision}</small></span>
        <span>{item.percent}%<br /><span className={`status-pill status-pill--${item.enabled ? "ok" : "not_wired"}`}>{item.enabled ? "Включён" : "Выключен"}</span></span>
        <span>{item.starts_at}<br />{item.ends_at ?? "Без окончания"}</span>
        <span>{item.plan_codes.join(", ")}</span>
        <span>В резерве: {item.reserved_count}<br />Использовано: {item.redeemed_count}<br />Лимит: {item.max_uses || "∞"}</span>
        <span className="retention-actions">{canManage ? <button type="button" disabled={busy} onClick={() => onEdit(item)}>Редактировать</button> : "Только просмотр"}</span>
      </div>)}
    </div> : <p className="muted" role="status">{busy ? "Загружаю промокоды…" : "Промокодов на этой странице нет."}</p>}
    <div className="retention-actions">
      <button type="button" disabled={busy || offset === 0} onClick={() => onPage(Math.max(0, offset - 50))}>Назад</button>
      <span>Страница {Math.floor(offset / 50) + 1}</span>
      <button type="button" disabled={busy || !hasMore} onClick={() => onPage(offset + 50)}>Далее</button>
    </div>
  </section>;
}

export function PromotionEditor({ existing, locked, onPrepare, onCancel }: { existing?: Promotion; locked: boolean; onPrepare: (body: PromotionConfig) => void; onCancel: () => void }) {
  const [draft, setDraft] = useState<PromotionDraft>(() => ({
    code: existing?.code ?? "", percent: existing ? String(existing.percent) : "", enabled: existing?.enabled ?? false,
    startsAt: existing?.starts_at ?? new Date().toISOString(), endsAt: existing?.ends_at ?? "",
    maxUses: String(existing?.max_uses ?? 0), planCodes: [...(existing?.plan_codes ?? [])],
  }));
  const [error, setError] = useState("");
  function submit(event: FormEvent) {
    event.preventDefault();
    if (locked) return;
    try { const body = buildPromotionConfig(draft, existing); setError(""); onPrepare(body); }
    catch (reason) { setError(reason instanceof Error ? reason.message : "Проверьте условия промокода."); }
  }
  function field(key: keyof PromotionDraft, value: string | boolean | string[]) { setDraft((current) => ({ ...current, [key]: value })); }
  return <form className="surface filters-panel promotion-form" onSubmit={submit} aria-label="Условия промокода">
    <div className="promotion-form__wide"><h3>{existing ? "Изменить промокод" : "Создать промокод"}</h3>{existing ? <p className="muted">Версия {existing.revision}. Код неизменяем.</p> : null}</div>
    <label><span>Код</span><input name="code" autoComplete="off" value={draft.code} readOnly={Boolean(existing)} disabled={locked} maxLength={64} onChange={(event) => field("code", event.target.value)} required /></label>
    <label><span>Скидка, % (1–99)</span><input name="percent" type="number" inputMode="numeric" min="1" max="99" step="1" value={draft.percent} disabled={locked} onChange={(event) => field("percent", event.target.value)} required /></label>
    <label><span>Начало (RFC 3339)</span><input name="starts_at" value={draft.startsAt} disabled={locked} onChange={(event) => field("startsAt", event.target.value)} required /><small>Часовой пояс обязателен; Z означает UTC.</small></label>
    <label><span>Окончание (необязательно)</span><input name="ends_at" value={draft.endsAt} disabled={locked} placeholder="2026-12-31T00:00:00Z" onChange={(event) => field("endsAt", event.target.value)} /><small>В момент окончания промокод уже не действует.</small></label>
    <label><span>Лимит применений</span><input name="max_uses" type="number" inputMode="numeric" min="0" step="1" value={draft.maxUses} disabled={locked} onChange={(event) => field("maxUses", event.target.value)} required /><small>0 — без ограничений</small></label>
    <label className="promotion-checkbox"><input name="enabled" type="checkbox" checked={draft.enabled} disabled={locked} onChange={(event) => field("enabled", event.target.checked)} /><span>Промокод включён</span></label>
    <fieldset className="promotion-form__wide promotion-plans" disabled={locked}><legend>Тарифы (хотя бы один)</legend>{promotionPlans.map((plan) => <label className="promotion-checkbox" key={plan}><input type="checkbox" name="plan_codes" value={plan} checked={draft.planCodes.includes(plan)} onChange={(event) => field("planCodes", event.target.checked ? [...draft.planCodes, plan] : draft.planCodes.filter((value) => value !== plan))} /><span>{plan}</span></label>)}</fieldset>
    {error ? <p className="promotion-form__wide" role="alert">{error}</p> : null}
    <div className="filter-actions promotion-form__wide"><button type="submit" disabled={locked}>Проверить изменения</button><button className="button-secondary" type="button" disabled={locked} onClick={onCancel}>Отменить изменения</button></div>
  </form>;
}

export function promotionMutationError(error: unknown): string {
  const safe = toSafeAdminError(error);
  if (safe.status === 409) return "Конфликт кода, версии или запроса. Обновите список и проверьте актуальные условия.";
  if (safe.status === 400) return "Условия промокода не приняты. Проверьте код, даты, скидку, тарифы и лимит.";
  if (safe.status === 401 || safe.status === 403) return "Недостаточно прав. Войдите с разрешением на промокоды.";
  if (safe.status === 404) return "Промокод не найден. Обновите список.";
  return "Результат запроса не подтверждён. Повторите сохранение с теми же условиями; ключ запроса сохранён до завершения действия.";
}
