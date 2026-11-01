export const promotionText = {
  ru: {
    description: "Промокод даёт процентную скидку на первую покупку тарифа.",
    applied: (code: string, percent: number) => `Промокод ${code} · −${percent}%`,
    remove: "Убрать промокод", checking: "Проверяем промокод…",
    invalid: "Промокод недействителен или срок его действия истёк.",
    ineligible: "Промокод недоступен для этого аккаунта или лимит использований исчерпан. Скидка действует только на первую покупку тарифа.",
    unavailable: "Не удалось проверить промокод. Повторите попытку.",
    preview: "Локальный предпросмотр: промокоды не применяются и оплата отключена.",
    signIn: "Войдите в аккаунт, чтобы применить промокод.",
    terms: "Скидка применяется к первой оплате подходящего тарифа. Количество звёзд не меняется. Продление — по обычной цене.",
  },
  en: {
    description: "A promo code gives a percentage discount on the first plan purchase.",
    applied: (code: string, percent: number) => `Promo code ${code} · −${percent}%`,
    remove: "Remove promo code", checking: "Checking promo code…",
    invalid: "This promo code is invalid or expired.",
    ineligible: "This promo code is unavailable for this account or its usage limit has been reached. The discount applies only to the first plan purchase.",
    unavailable: "Could not check the promo code. Try again.",
    preview: "Local preview: promo codes and checkout are disabled.",
    signIn: "Sign in to apply a promo code.",
    terms: "The discount applies to the first payment for an eligible plan. Included stars stay the same. Renewals use the regular price.",
  },
};
