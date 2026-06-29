import { useEffect, useState } from "react";
import type { AdminApiError, AdminClient } from "../api/adminClient";
import { toSafeAdminError } from "../api/adminClient";
import type {
  OperatorConfigFlagDTO,
  OperatorConfigHealthDTO,
  OperatorMediaPolicyDTO,
  OperatorMediaSafetyDTO,
  OperatorProviderControlRoomDTO,
  OperatorProviderHealthDTO,
  OperatorRiskSignalDTO,
  OperatorVideoRouteDTO,
} from "../api/providerMedia";

type OperatorScreenProps = {
  adminTokenSet: boolean;
  client: AdminClient;
};

type LoadState<T> = {
  data?: T;
  error?: AdminApiError;
  loading: boolean;
};

export function ProvidersScreen({ adminTokenSet, client }: OperatorScreenProps) {
  const state = useOperatorData<OperatorProviderControlRoomDTO>(
    adminTokenSet,
    client,
    "/admin/providers/operator",
  );

  if (!adminTokenSet) {
    return <LockedPanel title="Провайдеры закрыты" text="Введите админский токен, чтобы загрузить состояние провайдеров." />;
  }
  if (state.error) {
    return <SafeErrorPanel error={state.error} />;
  }
  if (!state.data) {
    return <LoadingPanel loading={state.loading} title="Loading provider control room" />;
  }

  return (
    <div className="ops-stack">
      <section className="surface queue-panel" aria-label="Provider risk summary">
        <div className="section-heading">
          <div>
            <p className="eyebrow">Providers</p>
            <h3>Control room</h3>
          </div>
          <span>{state.loading ? "Refreshing" : `Generated ${formatDateTime(state.data.generated_at)}`}</span>
        </div>
        <SignalGrid
          signals={[
            {
              id: "fallback",
              title: "Fallback health",
              status: state.data.fallback.status,
              value: state.data.fallback.provider_classes?.join(", ") || "none",
              source: "runtime_config_snapshot",
              summary: state.data.fallback.summary,
            },
            state.data.provider_waste,
            state.data.delivery_capture_gap,
            {
              id: "circuit",
              title: "Circuit state",
              status: state.data.circuit.status,
              value: state.data.circuit.status,
              source: state.data.circuit.source,
              summary: state.data.circuit.summary,
            },
          ]}
        />
      </section>

      <section className="surface jobs-list" aria-label="Provider and model class health">
        <div className="section-heading">
          <div>
            <p className="eyebrow">Curated classes</p>
            <h3>{`${state.data.providers.length} provider classes`}</h3>
          </div>
          <span>read-only</span>
        </div>
        <ProviderHealthTable items={state.data.providers} />
      </section>

      <section className="surface jobs-list" aria-label="Video route state">
        <div className="section-heading">
          <div>
            <p className="eyebrow">Video routes</p>
            <h3>{`${state.data.video_routes.length} route aliases`}</h3>
          </div>
          <span>model ids omitted</span>
        </div>
        <VideoRouteTable items={state.data.video_routes} />
      </section>

      <NotesPanel notes={state.data.notes} />
    </div>
  );
}

export function MediaSafetyScreen({ adminTokenSet, client }: OperatorScreenProps) {
  const state = useOperatorData<OperatorMediaSafetyDTO>(adminTokenSet, client, "/admin/media-safety/operator");

  if (!adminTokenSet) {
    return <LockedPanel title="Безопасность медиа закрыта" text="Введите админский токен, чтобы загрузить политики медиа и сигналы риска." />;
  }
  if (state.error) {
    return <SafeErrorPanel error={state.error} />;
  }
  if (!state.data) {
    return <LoadingPanel loading={state.loading} title="Loading media safety" />;
  }

  return (
    <div className="ops-stack">
      <MediaPolicyPanel policy={state.data.policy} generatedAt={state.data.generated_at} loading={state.loading} />
      <section className="surface queue-panel" aria-label="Media queue pressure">
        <div className="section-heading">
          <div>
            <p className="eyebrow">Queue pressure</p>
            <h3>{`Degradation: ${state.data.queue.degradation_state}`}</h3>
          </div>
          <span>{formatDateTime(state.data.queue.generated_at)}</span>
        </div>
        <div className="queue-metrics">
          {state.data.queue.backlog.map((metric) => (
            <div className={`queue-metric queue-metric--${metric.status}`} key={metric.label}>
              <span>{metric.label}</span>
              <strong>{metric.value}</strong>
            </div>
          ))}
          <div className="queue-metric queue-metric--not_wired">
            <span>oldest queued</span>
            <strong>
              {state.data.queue.oldest_queued_age_seconds === undefined
                ? "none"
                : formatDuration(state.data.queue.oldest_queued_age_seconds)}
            </strong>
          </div>
        </div>
        <p className="muted">{state.data.queue.provider_circuit.reason}</p>
      </section>
      <SignalSection title="Upload rejects" eyebrow="Validation" signals={state.data.uploads} />
      <SignalSection
        title="Processing policy"
        eyebrow="Probe / transcode / fast path"
        signals={[...state.data.processing, state.data.cleanup]}
      />
      <NotesPanel notes={state.data.notes} />
    </div>
  );
}

export function ConfigHealthScreen({ adminTokenSet, client }: OperatorScreenProps) {
  const state = useOperatorData<OperatorConfigHealthDTO>(adminTokenSet, client, "/admin/config-health/operator");

  if (!adminTokenSet) {
    return <LockedPanel title="Состояние конфига закрыто" text="Введите админский токен, чтобы загрузить несекретные runtime-флаги." />;
  }
  if (state.error) {
    return <SafeErrorPanel error={state.error} />;
  }
  if (!state.data) {
    return <LoadingPanel loading={state.loading} title="Loading config health" />;
  }

  return (
    <div className="ops-stack">
      <section className="surface jobs-list" aria-label="Config health">
        <div className="section-heading">
          <div>
            <p className="eyebrow">Non-secret flags</p>
            <h3>{`Environment: ${state.data.environment}`}</h3>
          </div>
          <span>{state.loading ? "Refreshing" : `Generated ${formatDateTime(state.data.generated_at)}`}</span>
        </div>
        <ConfigFlagTable flags={state.data.flags} />
      </section>

      <section className="surface jobs-list" aria-label="Runtime provider classes">
        <div className="section-heading">
          <div>
            <p className="eyebrow">Provider classes</p>
            <h3>{`${state.data.provider_classes.length} curated classes`}</h3>
          </div>
          <span>raw models omitted</span>
        </div>
        <div className="event-list">
          {state.data.provider_classes.map((item) => (
            <div key={`${item.provider_class}:${item.model_class}:${item.modality}`}>
              <strong>{`${item.provider_class} / ${item.model_class}`}</strong>
              <span>{item.modality}</span>
              <span>{item.contract_configured ? "contract configured" : "runtime default"}</span>
            </div>
          ))}
        </div>
      </section>

      <section className="surface jobs-list" aria-label="Runtime video routes">
        <div className="section-heading">
          <div>
            <p className="eyebrow">Video routes</p>
            <h3>{`${state.data.video_routes.length} route aliases`}</h3>
          </div>
          <span>flags and config only</span>
        </div>
        <VideoRouteTable items={state.data.video_routes} />
      </section>

      <NotesPanel notes={state.data.notes} />
    </div>
  );
}

function useOperatorData<T>(adminTokenSet: boolean, client: AdminClient, path: `/admin/${string}`): LoadState<T> {
  const [state, setState] = useState<LoadState<T>>({ loading: false });

  useEffect(() => {
    if (!adminTokenSet) {
      setState({ loading: false });
      return;
    }
    const controller = new AbortController();
    setState((current) => ({ data: current.data, loading: true }));
    client
      .request<T>(path, { signal: controller.signal })
      .then((data) => setState({ data, loading: false }))
      .catch((error: unknown) => {
        if (!controller.signal.aborted) {
          setState({ error: toSafeAdminError(error), loading: false });
        }
      });
    return () => controller.abort();
  }, [adminTokenSet, client, path]);

  return state;
}

function ProviderHealthTable({ items }: { items: OperatorProviderHealthDTO[] }) {
  if (items.length === 0) {
    return <p className="muted">No provider classes are configured in the safe snapshot.</p>;
  }
  return (
    <div className="provider-table" role="table">
      <div className="provider-row provider-row--head" role="row">
        <span>Provider</span>
        <span>Service</span>
        <span>Model class</span>
        <span>Health</span>
        <span>Errors</span>
        <span>Latency</span>
        <span>Guards</span>
      </div>
      {items.map((item) => (
        <div className="provider-row" key={`${item.service_type}:${item.provider_class}:${item.model_class}:${item.modality}`} role="row">
          <span>{item.provider_class}</span>
          <span>{providerServiceText(item)}</span>
          <span>{item.model_class}</span>
          <span className={`status-pill status-pill--${item.health}`}>{statusText(item.health)}</span>
          <span>{providerErrorsText(item)}</span>
          <span>{providerLatencyText(item)}</span>
          <span>{providerGuardsText(item)}</span>
        </div>
      ))}
    </div>
  );
}

function VideoRouteTable({ items }: { items: OperatorVideoRouteDTO[] }) {
  if (items.length === 0) {
    return <p className="muted">No video route aliases are present in the safe snapshot.</p>;
  }
  return (
    <div className="provider-table" role="table">
      <div className="provider-row provider-row--video-route provider-row--head" role="row">
        <span>Alias</span>
        <span>Provider</span>
        <span>Model class</span>
        <span>Status</span>
        <span>Reason</span>
        <span>Flags</span>
        <span>Inputs</span>
      </div>
      {items.map((item) => (
        <div className="provider-row provider-row--video-route" key={item.alias} role="row">
          <span>{item.alias}</span>
          <span>{item.provider_class}</span>
          <span>{item.model_class}</span>
          <span className={`status-pill status-pill--${item.status}`}>{statusText(item.status)}</span>
          <span>{item.reason}</span>
          <span>{routeReadinessText(item)}</span>
          <span>{routeInputText(item)}</span>
        </div>
      ))}
    </div>
  );
}

function MediaPolicyPanel({ policy, generatedAt, loading }: { policy: OperatorMediaPolicyDTO; generatedAt: string; loading: boolean }) {
  const metrics = [
    { label: "pipeline", value: boolLabel(policy.pipeline_enabled) },
    { label: "probe", value: policy.probe_policy },
    { label: "transcode", value: policy.transcode_policy },
    { label: "raw provider video", value: policy.raw_provider_video_policy },
    { label: "reference uploads", value: boolLabel(policy.reference_uploads_enabled) },
    { label: "webp references", value: boolLabel(policy.webp_reference_enabled) },
    { label: "image max", value: formatBytes(policy.max_image_upload_bytes) },
    { label: "pixels max", value: formatNumber(policy.max_image_pixels) },
    { label: "video max", value: formatBytes(policy.max_video_size_bytes) },
    { label: "video duration", value: `${policy.max_video_duration_sec}s` },
    { label: "upload concurrency", value: String(policy.max_concurrent_uploads) },
    { label: "probe concurrency", value: String(policy.max_concurrent_probes) },
    { label: "transcode concurrency", value: String(policy.max_concurrent_transcodes) },
    { label: "pending variants", value: String(policy.max_pending_variants) },
    { label: "queue degrade", value: String(policy.queue_degrade_threshold) },
    { label: "provider attempts", value: String(policy.provider_max_attempts_per_job) },
    { label: "fallback budget", value: String(policy.provider_fallback_budget_per_job) },
    { label: "quality guard", value: boolLabel(policy.provider_quality_guard_enabled) },
  ];
  return (
    <section className="surface queue-panel" aria-label="Media policy">
      <div className="section-heading">
        <div>
          <p className="eyebrow">Media policy</p>
          <h3>Worker-owned safety config</h3>
        </div>
        <span>{loading ? "Refreshing" : `Generated ${formatDateTime(generatedAt)}`}</span>
      </div>
      <div className="config-grid">
        {metrics.map((metric) => (
          <div className="queue-metric" key={metric.label}>
            <span>{metric.label}</span>
            <strong>{metric.value}</strong>
          </div>
        ))}
      </div>
    </section>
  );
}

function SignalSection({ title, eyebrow, signals }: { title: string; eyebrow: string; signals: OperatorRiskSignalDTO[] }) {
  return (
    <section className="surface queue-panel" aria-label={title}>
      <div className="section-heading">
        <div>
          <p className="eyebrow">{eyebrow}</p>
          <h3>{title}</h3>
        </div>
        <span>bounded labels</span>
      </div>
      <SignalGrid signals={signals} />
    </section>
  );
}

function SignalGrid({ signals }: { signals: OperatorRiskSignalDTO[] }) {
  return (
    <div className="signal-grid">
      {signals.map((signal) => (
        <article className={`signal-card signal-card--${signal.status}`} key={signal.id}>
          <div>
            <p className="eyebrow">{statusText(signal.status)}</p>
            <h4>{signal.title}</h4>
          </div>
          <strong>{signal.value}</strong>
          <p>{signal.summary}</p>
          <span>{signal.source}</span>
        </article>
      ))}
    </div>
  );
}

function ConfigFlagTable({ flags }: { flags: OperatorConfigFlagDTO[] }) {
  return (
    <div className="provider-table" role="table">
      <div className="provider-row provider-row--head" role="row">
        <span>Flag</span>
        <span>Value</span>
        <span>Status</span>
        <span>Summary</span>
      </div>
      {flags.map((flag) => (
        <div className="provider-row provider-row--config" key={flag.key} role="row">
          <span>{flag.key}</span>
          <span>{flag.value}</span>
          <span className={`status-pill status-pill--${flag.status}`}>{statusText(flag.status)}</span>
          <span>{flag.summary}</span>
        </div>
      ))}
    </div>
  );
}

function NotesPanel({ notes }: { notes?: string[] }) {
  if (!notes || notes.length === 0) {
    return null;
  }
  return (
    <section className="surface panel panel--wide" aria-label="Operator notes">
      <p className="eyebrow">Notes</p>
      {notes.map((note) => (
        <p className="muted" key={note}>
          {note}
        </p>
      ))}
    </section>
  );
}

function LockedPanel({ title, text }: { title: string; text: string }) {
  return (
    <article className="surface panel panel--wide" role="status">
      <p className="eyebrow">Нужен доступ</p>
      <h3>{title}</h3>
      <p>{text}</p>
    </article>
  );
}

function LoadingPanel({ loading, title }: { loading: boolean; title: string }) {
  return (
    <article className="surface panel panel--wide" role="status">
      <p className="eyebrow">{loading ? "Loading" : "No data"}</p>
      <h3>{title}</h3>
      <p>{loading ? "Requesting a bounded safe operator snapshot." : "The admin API has not returned data yet."}</p>
    </article>
  );
}

function SafeErrorPanel({ error }: { error: AdminApiError }) {
  return (
    <article className="surface panel panel--wide" role="alert">
      <p className="eyebrow">Безопасная ошибка</p>
      <h3>{error.message}</h3>
      <p>Код: {error.code}</p>
    </article>
  );
}

function statusText(status: string): string {
  if (status === "ok") {
    return "OK";
  }
  if (status === "warning") {
    return "Warning";
  }
  if (status === "critical") {
    return "Critical";
  }
  if (status === "unknown") {
    return "Unknown";
  }
  return "Not wired";
}

function boolLabel(value: boolean): string {
  return value ? "enabled" : "disabled";
}

function routeReadinessText(item: OperatorVideoRouteDTO): string {
  const bits = [
    item.enabled ? "route on" : "route off",
    item.provider_enabled ? "provider on" : "provider off",
    item.provider_configured && item.provider_base_configured ? "configured" : "unconfigured",
    item.cost_configured ? "cost ok" : "cost missing",
  ];
  return bits.join(" / ");
}

function routeInputText(item: OperatorVideoRouteDTO): string {
  const duration = item.allowed_durations_sec?.length ? `${item.allowed_durations_sec.join(",")}s` : "duration n/a";
  const resolution = item.allowed_resolutions?.length ? item.allowed_resolutions.join(",") : "resolution n/a";
  const refs = item.supports_reference_image ? `refs ${item.max_reference_images || 1}` : "no refs";
  return [duration, resolution, item.requires_start_image ? "start image" : "text ok", refs].join(" / ");
}

function providerServiceText(item: OperatorProviderHealthDTO): string {
  return `${item.service_type || "ai_provider"} / ${item.modality}`;
}

function providerErrorsText(item: OperatorProviderHealthDTO): string {
  const latest = item.latest_error_class ? ` / latest ${item.latest_error_class}` : "";
  return `${formatPercent(item.error_rate_percent)} / failed ${item.provider_failed_count} / rate ${item.rate_limit_count} / invalid ${item.invalid_output_count}${latest}`;
}

function providerLatencyText(item: OperatorProviderHealthDTO): string {
  const latency = item.latency_p95_ms > 0 ? `p95 ${item.latency_p95_ms} ms` : "p95 n/a";
  return `${latency} / in-flight ${item.in_flight_count} / samples ${item.observed_total_count}`;
}

function providerGuardsText(item: OperatorProviderHealthDTO): string {
  const latestAt = item.latest_error_at ? ` / ${formatDateTime(item.latest_error_at)}` : "";
  return `quota ${item.quota_state} / cooldown ${item.cooldown_state} / circuit ${item.circuit_state} / fallback ${item.fallback_state}${latestAt}`;
}

function formatDateTime(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "unknown";
  }
  return date.toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short" });
}

function formatDuration(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds < 0) {
    return "unknown";
  }
  if (seconds < 60) {
    return `${Math.round(seconds)}s`;
  }
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) {
    return `${minutes}m`;
  }
  const hours = Math.floor(minutes / 60);
  return `${hours}h`;
}

function formatBytes(value: number): string {
  if (!Number.isFinite(value) || value < 0) {
    return "unknown";
  }
  if (value < 1024) {
    return `${value} B`;
  }
  const units = ["KB", "MB", "GB"];
  let amount = value / 1024;
  for (const unit of units) {
    if (amount < 1024) {
      return `${amount.toFixed(amount >= 10 ? 0 : 1)} ${unit}`;
    }
    amount /= 1024;
  }
  return `${amount.toFixed(0)} TB`;
}

function formatNumber(value: number): string {
  if (!Number.isFinite(value)) {
    return "unknown";
  }
  return new Intl.NumberFormat().format(value);
}

function formatPercent(value: number): string {
  if (!Number.isFinite(value)) {
    return "unknown";
  }
  return `${value.toFixed(value >= 10 ? 0 : 1)}%`;
}
