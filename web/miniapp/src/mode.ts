export type AppTab = "create" | "chat" | "settings";

const TAB_KEY = "vk_miniapp_active_tab_v1";

export function loadAppTab(): AppTab {
  try {
    const value = localStorage.getItem(TAB_KEY);
    if (value === "create" || value === "settings") return value;
    return "chat";
  } catch {
    return "chat";
  }
}

export function saveAppTab(tab: AppTab): void {
  try {
    localStorage.setItem(TAB_KEY, tab);
  } catch {
    /* UI preference only */
  }
}
