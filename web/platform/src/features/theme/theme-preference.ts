export const themeStorageKey = "neirohub.theme";
export const themePreferences = ["system", "light", "dark"] as const;

export type ThemePreference = (typeof themePreferences)[number];

function isThemePreference(value: string | null): value is ThemePreference {
  return themePreferences.some((preference) => preference === value);
}

export function readThemePreference(): ThemePreference {
  try {
    const value = window.localStorage.getItem(themeStorageKey);
    return isThemePreference(value) ? value : "system";
  } catch {
    return "system";
  }
}

export function applyThemePreference(preference: ThemePreference): void {
  document.documentElement.dataset.theme = preference;

  try {
    window.localStorage.setItem(themeStorageKey, preference);
  } catch {
    // Storage can be unavailable in privacy-restricted browsers. The current
    // document still keeps the selected theme through its root attribute.
  }
}

export const themeBootstrapScript =
  `(()=>{try{const value=window.localStorage.getItem("${themeStorageKey}");` +
  `document.documentElement.dataset.theme=value==="system"||value==="light"||value==="dark"?value:"system";` +
  `}catch{document.documentElement.dataset.theme="system";}})();`;
