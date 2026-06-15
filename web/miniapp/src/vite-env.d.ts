/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_DEV_LAUNCH_PARAMS?: string;
  readonly VITE_FRONTEND_TELEMETRY_ENABLED?: string;
  readonly VITE_FEATURE_MINIAPP_PAYMENT_CANCEL_ENABLED?: string;
  readonly VITE_FEATURE_MINIAPP_TOPUP_CATALOG_DROPDOWN_ENABLED?: string;
  readonly VITE_FEATURE_MINIAPP_DARK_THEME_ONLY_ENABLED?: string;
  readonly VITE_FEATURE_MINIAPP_TOPUP_HISTORY_DROPDOWN_ENABLED?: string;
}
