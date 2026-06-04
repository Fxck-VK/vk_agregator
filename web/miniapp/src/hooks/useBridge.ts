// src/hooks/useBridge.ts
import { useEffect, useState } from "react";
import bridge from "@vkontakte/vk-bridge";

// Инициализирует VK Bridge. Вне VK (прямой заход в браузере) промис
// отклонится — это нормально, мы всё равно показываем интерфейс.
export function useBridgeInit(): boolean {
  const [ready, setReady] = useState(false);
  useEffect(() => {
    bridge
      .send("VKWebAppInit")
      .catch(() => {})
      .finally(() => setReady(true));
  }, []);
  return ready;
}