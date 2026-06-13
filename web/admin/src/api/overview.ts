export type OverviewCardStatus = "ok" | "warning" | "critical" | "not_wired";

export type OverviewMetricDTO = {
  label: string;
  value: string;
  status?: OverviewCardStatus;
};

export type OverviewCardDTO = {
  id: string;
  title: string;
  status: OverviewCardStatus;
  summary: string;
  metrics?: OverviewMetricDTO[];
};

export type OverviewDTO = {
  generated_at: string;
  cards: OverviewCardDTO[];
};
