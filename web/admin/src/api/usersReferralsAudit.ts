import type { PaginationDTO } from "./jobs";

export type OperatorUserJobSummaryDTO = {
  status: string;
  total: string;
  active: string;
  succeeded: string;
  failed: string;
  text_jobs: string;
  image_jobs: string;
  video_jobs: string;
  recent_page_size: number;
};

export type OperatorUserSummaryDTO = {
  user_ref: string;
  role: string;
  status: string;
  locale?: string;
  risk_class: string;
  first_seen_at?: string;
  last_seen_at?: string;
  created_at: string;
  updated_at: string;
  age_seconds: number;
  jobs: OperatorUserJobSummaryDTO;
};

export type OperatorUserRecentJobDTO = {
  display_id: string;
  operation: string;
  modality: string;
  status: string;
  error_class?: string;
  cost_reserved: number;
  cost_captured: number;
  created_at: string;
  age_seconds: number;
};

export type OperatorUserPaymentSummaryDTO = {
  status: string;
  total: number;
  pending: number;
  succeeded: number;
  failed: number;
  refunded: number;
  credits_purchased: number;
};

export type OperatorUserReferralSummaryDTO = {
  status: string;
  code?: string;
  invited: number;
  registered: number;
  activated: number;
  rewarded: number;
  invited_by?: {
    source: string;
    status: string;
    reward_status: string;
  };
};

export type OperatorUsersDTO = {
  generated_at: string;
  user?: OperatorUserSummaryDTO;
  recent_jobs?: OperatorUserRecentJobDTO[];
  payment: OperatorUserPaymentSummaryDTO;
  referrals: OperatorUserReferralSummaryDTO;
  notes?: string[];
};

export type ReferralStatsDTO = {
  code: string;
  invited_count: number;
  registered_count: number;
  activated_count: number;
  rewarded_count: number;
};

export type SuspiciousReferralDTO = ReferralStatsDTO & {
  reasons: string[];
};

export type OperatorReferralsDTO = {
  generated_at: string;
  code_stats?: ReferralStatsDTO;
  distribution: {
    registered_count: number;
    activated_count: number;
    rewarded_count: number;
    total: number;
  };
  suspicious: SuspiciousReferralDTO[];
  suspicious_criteria: {
    min_registered: number;
    min_total: number;
  };
  pagination: PaginationDTO;
  notes?: string[];
};

export type OperatorAuditEntryDTO = {
  display_id: string;
  actor_ref: string;
  action: string;
  target_type: string;
  target_ref?: string;
  result: "success" | "error";
  request_ref?: string;
  created_at: string;
};

export type OperatorAuditLogDTO = {
  generated_at: string;
  items: OperatorAuditEntryDTO[];
  pagination: PaginationDTO;
  notes?: string[];
};
