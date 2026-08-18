"use client";

import { useState } from "react";

import {
  applyThemePreference,
  readThemePreference,
  type ThemePreference,
} from "@/features/theme/theme-preference";

import styles from "./PublicThemeSwitcher.module.css";

type PublicThemeSwitcherProps = {
  labels: {
    dark: string;
    group: string;
    light: string;
    system: string;
  };
};

const themeOptions: ReadonlyArray<{ icon: string; key: ThemePreference }> = [
  { icon: "▣", key: "system" },
  { icon: "☀", key: "light" },
  { icon: "●", key: "dark" },
];

export function PublicThemeSwitcher({ labels }: PublicThemeSwitcherProps) {
  const [preference, setPreference] = useState<ThemePreference>(() =>
    typeof window === "undefined" ? "system" : readThemePreference(),
  );

  const selectTheme = (nextPreference: ThemePreference) => {
    applyThemePreference(nextPreference);
    setPreference(nextPreference);
  };

  return (
    <div aria-label={labels.group} className={styles.root} role="group">
      {themeOptions.map((option) => (
        <button
          aria-label={labels[option.key]}
          aria-pressed={preference === option.key}
          className={styles.option}
          data-selected={preference === option.key}
          key={option.key}
          onClick={() => selectTheme(option.key)}
          type="button"
        >
          <span aria-hidden="true">{option.icon}</span>
        </button>
      ))}
    </div>
  );
}
