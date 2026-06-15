// src/main.tsx
import React, { useEffect, useState } from "react";
import { createRoot } from "react-dom/client";
import { AdaptivityProvider, AppRoot, ConfigProvider } from "@vkontakte/vkui";
import App from "./App";
import { MINIAPP_DARK_THEME_ONLY_ENABLED, installFrontendTelemetry } from "./api/client";
import { applyInitialThemeMode } from "./settings/theme";
import "@vkontakte/vkui/dist/vkui.css";
import "./ui/theme.css";

type VkuiColorScheme = "light" | "dark";

function readColorScheme(): VkuiColorScheme {
  if (MINIAPP_DARK_THEME_ONLY_ENABLED) return "dark";
  const attr = document.documentElement.getAttribute("data-scheme");
  if (attr === "dark" || attr === "light") return attr;
  if (window.matchMedia?.("(prefers-color-scheme: dark)").matches) return "dark";
  return "light";
}

function RootProviders() {
  const [colorScheme, setColorScheme] = useState<VkuiColorScheme>(() => readColorScheme());

  useEffect(() => {
    const update = () => setColorScheme(readColorScheme());
    const observer = new MutationObserver(update);
    observer.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ["data-scheme"],
    });

    const media = window.matchMedia?.("(prefers-color-scheme: dark)");
    media?.addEventListener("change", update);

    return () => {
      observer.disconnect();
      media?.removeEventListener("change", update);
    };
  }, []);

  return (
    <ConfigProvider colorScheme={colorScheme} locale="ru" isWebView>
      <AdaptivityProvider>
        <AppRoot mode="full" className="vkui-miniapp" userSelectMode="enabled-with-pointer">
          <App />
        </AppRoot>
      </AdaptivityProvider>
    </ConfigProvider>
  );
}

applyInitialThemeMode();
installFrontendTelemetry();

createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <RootProviders />
  </React.StrictMode>,
);
