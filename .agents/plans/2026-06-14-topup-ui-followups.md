# План следующих задач Mini App top-up UI

Дата: 2026-06-14
Ветка: fastlife_dev

## Уже реализуется в текущем scope

- `FEATURE_VK_TOPUP_STATUS_EDIT_ENABLED`: VK-бот сохраняет id сообщения счета, webhook processor после verified status редактирует сообщение на успех/отклонение.
- `FEATURE_MINIAPP_PAYMENT_CANCEL_ENABLED`: Mini App получает кнопку отмены ожидающего платежа и backend endpoint, который отменяет только свой `vk_miniapp` intent через провайдера.

## Инварианты для всех следующих задач

- Frontend не решает баланс, статус платежа, цену, скидку, product code или ownership.
- Любая новая UI-фича должна иметь отдельный env flag backend/frontend.
- Все платежные действия идут через `paymentservice`; Mini App BFF не мутирует ledger.
- История и каталог показывают только DTO без `provider_payment_id`, metadata, user_id, receipt contacts.
- Логи: без токенов, launch params, provider payload, email/phone, raw metadata.

## 3. Выпадающий список пополнения / магазин цен

Флаг: `FEATURE_MINIAPP_TOPUP_CATALOG_DROPDOWN_ENABLED`

План:

1. Backend оставить источником истины: `GET /miniapp/payment-products` возвращает активный catalog как сейчас.
2. Frontend заменить grid/list tariff cards на select/dropdown, если флаг включен.
3. При выборе тарифа показывать amount, credits, title и одну кнопку создания платежа.
4. Не отправлять amount/credits с клиента; отправлять только `product_code`, receipt contact, `force_new`.
5. Покрыть тестом загрузку каталога и создание intent по выбранному product code.

Промт для реализации:

```text
Реализуй пункт 3: в Mini App сделай выпадающий список пополнения вместо списка карточек под флагом FEATURE_MINIAPP_TOPUP_CATALOG_DROPDOWN_ENABLED. Backend цену не принимает с клиента: используй только product_code из GET /miniapp/payment-products. Добавь frontend env VITE_FEATURE_MINIAPP_TOPUP_CATALOG_DROPDOWN_ENABLED, не ломай текущий UI при false. Добавь тесты на выбор тарифа и создание intent. Не трогай ledger/webhook.
```

## 4. Только темная тема, убрать выбор темы

Флаг: `FEATURE_MINIAPP_DARK_THEME_ONLY_ENABLED`

План:

1. В `settings/theme.ts` при включенном флаге всегда применять `dark`.
2. Не читать и не сохранять `light/system` в localStorage, если флаг включен.
3. В `SettingsScreen` скрыть блок выбора темы под этим же флагом.
4. Проверить, что CSS не зависит от `data-scheme="light"` как обязательного режима.
5. Добавить frontend unit/smoke test на отсутствие theme chooser и forced dark.

Промт для реализации:

```text
Реализуй пункт 4: в Mini App оставь только темную тему под флагом FEATURE_MINIAPP_DARK_THEME_ONLY_ENABLED / VITE_FEATURE_MINIAPP_DARK_THEME_ONLY_ENABLED. Убери видимый выбор темы, принудительно применяй dark, не сохраняй light/system при включенном флаге. Проверь mobile/desktop визуально, чтобы текст и кнопки не пересекались.
```

## 5. Выпадающий список истории пополнений

Флаг: `FEATURE_MINIAPP_TOPUP_HISTORY_DROPDOWN_ENABLED`

План:

1. Backend оставить текущий `GET /miniapp/payments` с pagination и source filter `vk_miniapp`.
2. Frontend показывать summary-row истории и раскрывать список по клику.
3. В collapsed state показывать последний платеж и счетчик.
4. В expanded state показывать последние N платежей, статусы, дату, amount/credits и link `Продолжить` только для `waiting_for_user`.
5. Если пункт 2 уже включен, для active waiting rows можно показать `Отменить`, но она должна вызывать тот же cancel endpoint.
6. Добавить frontend test на collapsed/expanded states и отсутствие provider/private fields.

Промт для реализации:

```text
Реализуй пункт 5: сделай историю пополнений в Mini App выпадающим списком под флагом FEATURE_MINIAPP_TOPUP_HISTORY_DROPDOWN_ENABLED / VITE_FEATURE_MINIAPP_TOPUP_HISTORY_DROPDOWN_ENABLED. Backend не меняй без необходимости: используй GET /miniapp/payments. В закрытом состоянии покажи последний платеж и количество, в открытом - список с safe DTO fields. Для active waiting оставь continue link и, если FEATURE_MINIAPP_PAYMENT_CANCEL_ENABLED включен, кнопку отмены через существующий endpoint.
```
