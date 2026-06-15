import type { PaymentProduct } from "../api/client";

export function selectedPaymentProduct(products: PaymentProduct[], selectedCode: string): PaymentProduct | null {
  return products.find((product) => product.code === selectedCode) ?? products[0] ?? null;
}

export function paymentHistoryCountLabel(count: number): string {
  if (count <= 0) return "Нет платежей";
  const lastTwo = count % 100;
  const last = count % 10;
  if (last === 1 && lastTwo !== 11) return `${count} платеж`;
  if (last >= 2 && last <= 4 && (lastTwo < 12 || lastTwo > 14)) return `${count} платежа`;
  return `${count} платежей`;
}
