import bridge from "@vkontakte/vk-bridge";

type UntypedBridge = {
  send(method: string, props?: Record<string, unknown>): Promise<unknown>;
  supports?(method: string): boolean;
  supportsAsync?(method: string): Promise<boolean>;
};

const externalBridge = bridge as unknown as UntypedBridge;

async function bridgeSupports(method: string): Promise<boolean> {
  try {
    if (typeof externalBridge.supportsAsync === "function") {
      return await externalBridge.supportsAsync(method);
    }
    if (typeof externalBridge.supports === "function") {
      return externalBridge.supports(method);
    }
  } catch {
    return true;
  }
  return true;
}

export function safeExternalHttpsUrl(url: string | null | undefined): string | null {
  const trimmed = url?.trim();
  if (!trimmed) return null;
  try {
    const parsed = new URL(trimmed);
    if (parsed.protocol !== "https:" || !parsed.hostname) return null;
    return parsed.toString();
  } catch {
    return null;
  }
}

export async function openExternalUrl(url: string): Promise<boolean> {
  const normalizedUrl = safeExternalHttpsUrl(url);
  if (!normalizedUrl) return false;

  try {
    if (await bridgeSupports("VKWebAppOpenLink")) {
      await externalBridge.send("VKWebAppOpenLink", { url: normalizedUrl });
      return true;
    }
  } catch {
    // Fall back to browser navigation below.
  }

  const popup = window.open(normalizedUrl, "_blank", "noopener,noreferrer");
  if (popup) {
    popup.opener = null;
    return true;
  }

  return false;
}
