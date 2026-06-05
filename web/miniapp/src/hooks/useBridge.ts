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

    (async () => {
      try {
        await bridge.send("VKWebAppInit");
      } catch {
        /* вне VK */
      }
      try {
        const cfg = (await bridge.send("VKWebAppGetConfig")) as {
          appearance?: string;
          scheme?: string;
        };
        applyScheme(cfg?.appearance ?? cfg?.scheme);
      } catch {
        /* нет конфига вне VK */
      }
      try {
        const info = (await bridge.send("VKWebAppGetUserInfo")) as {
          first_name?: string;
          last_name?: string;
          photo_100?: string;
          photo_200?: string;
        };
        if (active) {
          const first = info?.first_name ?? "";
          const last = info?.last_name ?? "";
          setUser({
            firstName: first || "друг",
            name: [first, last].filter(Boolean).join(" ") || "Гость",
            avatar: info?.photo_100 ?? info?.photo_200 ?? null,
          });
        }
      } catch {
        if (active) setUser({ firstName: "друг", name: "Гость", avatar: null });
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
