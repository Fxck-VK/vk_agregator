import { PaymentReturn } from "@/features/payments/PaymentReturn";

export default async function PaymentReturnPage({ searchParams }: { searchParams: Promise<{ payment_id?: string | string[] }> }) {
  const { payment_id } = await searchParams;
  return <PaymentReturn paymentID={typeof payment_id === "string" ? payment_id : undefined} />;
}
