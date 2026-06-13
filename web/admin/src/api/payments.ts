import type { PaginationDTO } from "./jobs";

export type OperatorPaymentIntentDTO = {
  display_id: string;
  action_ref?: string;
  user_ref: string;
  product_ref?: string;
  status: string;
  amount: number;
  currency: string;
  credits: number;
  provider: string;
  provider_payment_ref?: string;
  confirmation_state: string;
  capture_state: string;
  cancel_state: string;
  refund_state: string;
  stale: boolean;
  stale_seconds?: number;
  created_at: string;
  updated_at: string;
};

export type OperatorPaymentEventDTO = {
  display_id: string;
  provider: string;
  event_type: string;
  provider_payment_ref?: string;
  provider_refund_ref?: string;
  processed: boolean;
  processed_at?: string;
  received_at: string;
  updated_at: string;
};

export type OperatorPaymentRefundDTO = {
  display_id: string;
  intent_ref: string;
  provider_refund_ref?: string;
  amount: number;
  status: string;
  reason_present: boolean;
  created_at: string;
  updated_at: string;
};

export type OperatorPaymentReconciliationDTO = {
  status: string;
  pending_count: number;
  stale_count: number;
  unprocessed_event_count: number;
  refund_count: number;
  stale_after_seconds: number;
};

export type OperatorLedgerEntryDTO = {
  display_id: string;
  type: string;
  status: string;
  amount: number;
  job_ref?: string;
  reservation_ref?: string;
  reason_class?: string;
  created_at: string;
};

export type OperatorBillingReservationDTO = {
  display_id: string;
  job_ref: string;
  status: string;
  amount: number;
  expires_at: string;
  updated_at: string;
};

export type OperatorBillingDTO = {
  user_ref: string;
  balance_credits: number;
  ledger: OperatorLedgerEntryDTO[];
  reservations: OperatorBillingReservationDTO[];
};

export type OperatorPaymentsConsoleDTO = {
  generated_at: string;
  intents: OperatorPaymentIntentDTO[];
  events: OperatorPaymentEventDTO[];
  refunds: OperatorPaymentRefundDTO[];
  reconciliation: OperatorPaymentReconciliationDTO;
  billing?: OperatorBillingDTO;
  pagination: PaginationDTO;
};
