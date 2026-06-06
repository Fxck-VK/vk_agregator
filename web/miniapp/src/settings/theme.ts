export type ThemeMode = "system" | "light" | "dark";

const THEME_KEY = "vk_miniapp_theme_v1";

function isThemeMode(value: string | null): value is ThemeMode {
  return value === "system" || value === "light" || value === "dark";
}

export function loadThemeMode(): ThemeMode {
  try {
    const value = localStorage.getItem(THEME_KEY);
    return isThemeMode(value) ? value : "system";
  } catch {
    return "system";
  }
}

function saveThemeMode(mode: ThemeMode): void {
  try {
    localStorage.setItem(THEME_KEY, mode);
  } catch {
    /* UI preference only */
  }
}

export function applyInitialThemeMode(): void {
  const mode = loadThemeMode();
  document.documentElement.dataset.themeMode = mode;
  if (mode === "light" || mode === "dark") {
    document.documentElement.setAttribute("data-scheme", mode);
  }
}

export function watchThemeMode(mode: ThemeMode): () => void {
  const root = document.documentElement;
  saveThemeMode(mode);
  root.dataset.themeMode = mode;

  if (mode === "system") {
    root.removeAttribute("data-scheme");
    return () => undefined;
  }

  const apply = () => {
    if (root.getAttribute("data-scheme") !== mode) {
      root.setAttribute("data-scheme", mode);
    }
  };

  apply();
  const observer = new MutationObserver(apply);
  observer.observe(root, { attributes: true, attributeFilter: ["data-scheme"] });
  return () => observer.disconnect();
}
