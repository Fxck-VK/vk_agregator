import economy from "@/config/economy.generated.json";

const retries = economy.renewal_retries;

export const subscriptionCopy = {
  ru: {
    trialUsed: "Пробный тариф Lite уже использован. Выберите другой тариф.",
    title: "Подписка", otherGrant: "Начисление", adjustedBalance: "Корректировка баланса", choose: "Выбрать", days: "дней", period: "За оплаченный период", offer: "Условия подписки", pricingNote: "Цена и звёзды за указанный период. Доступность оплаты проверяется в аккаунте.", costs: "Число генераций зависит от стоимости выбранной операции. Точная цена показывается перед запуском.",
    loading: "Проверяем доступность подписок…", unavailable: "Оплата подписок сейчас недоступна.", preview: "Локальный просмотр: оплата и автопродление отключены.",
    sharedStars: "Единый баланс звёзд", checkoutHelp: "Нажмите «Выбрать тариф»: доступность оплаты проверяется в аккаунте. Перед оплатой показаны цена, звёзды и срок. Автопродление включается только с отдельного согласия.",
    checkoutNotice: "Перед оплатой ознакомьтесь с условиями оплаты и реквизитами. Продление выбирается выше.",
    signIn: "Войдите в аккаунт, чтобы проверить доступность оплаты.", error: "Не удалось проверить подписку. Повторите попытку.", retry: "Повторить", purchaseError: "Не удалось открыть оплату. Повторите попытку с теми же условиями.", conflict: "Условия изменились или подписка уже существует. Проверьте подписку в профиле.",
    pay: "Перейти к оплате", pending: "Открываем оплату…", consent: `Разрешаю автоматическое продление и регулярные списания по указанным условиям. При нехватке средств повторные попытки списания — с интервалом не менее ${retries.interval_hours} часов в течение ${retries.window_days} дней с первой неудачной попытки, не более ${retries.max_attempts} попыток всего, включая первую. После отключения автопродления новые попытки прекращаются.`,
    renewOff: "Автопродление выключено. Следующего автоматического списания не будет.", recurringOff: "Автоматическое продление сейчас недоступно.",
    expiry: "Срок действия неиспользованных звёзд этой подписки заканчивается в конце оплаченного периода. Ранее купленные пополнения учитываются отдельно.",
    trial: "Lite доступен один раз для аккаунта. При включённом автопродлении следующий период — Start+.",
    next: "Следующее списание при автопродлении", after: "после", for: "за", stars: "звёзд", authority: "Подписка и звёзды появятся только после подтверждения оплаты сервером.",
    noPlan: "Активной подписки нет", paidUntil: "Оплачено до", nextCharge: "Следующее списание", renewal: "Автопродление", on: "Включено", off: "Выключено", balances: "Из чего состоит баланс", available: "Доступно", reserved: "В резерве", expires: "Истекает", noExpiry: "Без срока подписки", empty: "Нет начислений для отображения.",
    cancel: "Отключить автопродление", cancelTitle: "Отключить автопродление?", cancelDescription: "Новых регулярных списаний не будет. Уже оплаченный период действует до указанной даты.", keep: "Оставить", cancelling: "Отключаем…", cancelError: "Не удалось отключить автопродление. Повторите попытку.",
    status: { active: "Действует", cancelled: "Продление отменено", expired: "Истекла", past_due: "Продление не оплачено" },
    lot: { subscription: "Подписка", purchased: "Купленные звёзды", topup: "Пополнение", legacy: "Прежний баланс", bonus: "Бонус", adjustment: "Корректировка" },
  },
  en: {
    trialUsed: "The Lite trial has already been used. Choose another plan.",
    title: "Subscription", otherGrant: "Grant", adjustedBalance: "Balance adjustment", choose: "Choose", days: "days", period: "For the paid period", offer: "Subscription terms", pricingNote: "Price and stars for the displayed period. Checkout availability is checked in your account.", costs: "Generation counts depend on the selected operation cost. The exact price is shown before starting.",
    loading: "Checking subscription availability…", unavailable: "Subscription checkout is currently unavailable.", preview: "Local preview: checkout and recurring payments are disabled.",
    sharedStars: "Shared star balance", checkoutHelp: "Choose a plan to check checkout availability in your account. The price, stars and period are shown before payment. Automatic renewal requires separate consent.",
    checkoutNotice: "Before payment, review the payment terms and payment details. Renewal is selected above.",
    signIn: "Sign in to check checkout availability.", error: "Could not check your subscription. Please retry.", retry: "Retry", purchaseError: "Could not open checkout. Retry with the same terms.", conflict: "Terms changed or a subscription already exists. Check your subscription in your profile.",
    pay: "Continue to payment", pending: "Opening checkout…", consent: `I authorize automatic renewal and recurring charges under the displayed terms. If funds are insufficient, payment attempts may repeat at intervals of at least ${retries.interval_hours} hours within ${retries.window_days} days of the first failed attempt, up to ${retries.max_attempts} attempts in total including the first. Turning off automatic renewal stops new attempts.`,
    renewOff: "Automatic renewal is off. There will be no next automatic charge.", recurringOff: "Automatic renewal is currently unavailable.",
    expiry: "Unused stars from this subscription expire at the end of the paid period. Previously purchased top-ups are accounted for separately.",
    trial: "Lite is available once per account. With automatic renewal enabled, the next period is Start+.",
    next: "Next charge with automatic renewal", after: "after", for: "for", stars: "stars", authority: "The subscription and stars are granted only after the server verifies payment.",
    noPlan: "No active subscription", paidUntil: "Paid until", nextCharge: "Next charge", renewal: "Automatic renewal", on: "On", off: "Off", balances: "Balance breakdown", available: "Available", reserved: "Reserved", expires: "Expires", noExpiry: "No subscription expiry", empty: "No grants to display.",
    cancel: "Turn off automatic renewal", cancelTitle: "Turn off automatic renewal?", cancelDescription: "There will be no new recurring charges. Your paid period remains available until the displayed date.", keep: "Keep it", cancelling: "Turning off…", cancelError: "Could not turn off automatic renewal. Please retry.",
    status: { active: "Active", cancelled: "Renewal cancelled", expired: "Expired", past_due: "Renewal unpaid" },
    lot: { subscription: "Subscription", purchased: "Purchased stars", topup: "Top-up", legacy: "Previous balance", bonus: "Bonus", adjustment: "Adjustment" },
  },
} as const;
