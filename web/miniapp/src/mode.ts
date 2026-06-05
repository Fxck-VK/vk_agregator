export type AppMode = "chat" | "workflow";

const MODE_KEY = "vk_miniapp_mode_v1";

export function loadAppMode(): AppMode {
  try {
    const value = localStorage.getItem(MODE_KEY);
    return value === "workflow" ? "workflow" : "chat";
  } catch {
    return "chat";
  }
}

export function saveAppMode(mode: AppMode): void {
  try {
    localStorage.setItem(MODE_KEY, mode);
  } catch {
    /* UI preference only */
  }
}
