// src/hooks/useBridge.ts
import { useEffect, useState } from "react";
import bridge from "@vkontakte/vk-bridge";

export interface VkUser {
  name: string;
  firstName: string;
  avatar: string | null;
}

function applyScheme(scheme?: string) {
  if (!scheme) return;
  const dark = /dark|space_gray/i.test(scheme);
  document.documentElement.setAttribute("data-scheme", dark ? "dark" : "light");
}

function bridgeTimeoutMs(): number {
  return import.meta.env.DEV ? 1200 : 3000;
}

function withTimeout<T>(promise: Promise<T>, timeoutMs: number): Promise<T> {
  let timer: number | undefined;
  const timeout = new Promise<T>((_, reject) => {
    timer = window.setTimeout(() => reject(new Error("vk bridge timeout")), timeoutMs);
  });
  return Promise.race([promise, timeout]).finally(() => {
    if (timer !== undefined) window.clearTimeout(timer);
  });
}

export function useBridge() {
  const [ready, setReady] = useState(false);
  const [user, setUser] = useState<VkUser | null>(null);

  useEffect(() => {
    let active = true;

    const handleConfigUpdate: Parameters<typeof bridge.subscribe>[0] = (event) => {
      if (event.detail.type !== "VKWebAppUpdateConfig") return;
      const data = event.detail.data as { appearance?: string; scheme?: string };
      applyScheme(data.appearance ?? data.scheme);
    };
    bridge.subscribe(handleConfigUpdate);

    void (async () => {
      const timeoutMs = bridgeTimeoutMs();
      try {
        await withTimeout(bridge.send("VKWebAppInit"), timeoutMs);
      } catch {
        /* вне VK */
      }
      try {
        const cfg = (await withTimeout(bridge.send("VKWebAppGetConfig"), timeoutMs)) as {
          appearance?: string;
          scheme?: string;
        };
        applyScheme(cfg?.appearance ?? cfg?.scheme);
      } catch {
        /* нет конфига вне VK */
      }
      try {
        const info = (await withTimeout(bridge.send("VKWebAppGetUserInfo"), timeoutMs)) as {
          first_name?: string;
          last_name?: string;
          photo_100?: string;
          photo_200?: string;
        };
        if (active) {
          const first = info?.first_name ?? "";
          const last = info?.last_name ?? "";
          setUser({
            firstName: first || "Пользователь",
            name: [first, last].filter(Boolean).join(" ") || "Пользователь",
            avatar: info?.photo_100 ?? info?.photo_200 ?? null,
          });
        }
      } catch {
        if (active) setUser({ firstName: "Пользователь", name: "Пользователь", avatar: null });
      }
      if (active) setReady(true);
    })();

    return () => {
      active = false;
      bridge.unsubscribe(handleConfigUpdate);
    };
  }, []);

  return { ready, user };
}

export function haptic(kind: "light" | "success" | "error" = "light") {
  try {
    if (kind === "light") {
      void bridge.send("VKWebAppTapticImpactOccurred", { style: "light" });
    } else {
      void bridge.send("VKWebAppTapticNotificationOccurred", { type: kind });
    }
  } catch {
    /* нет тактильной отдачи */
  }
}
