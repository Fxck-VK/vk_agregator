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

export async function openExternalUrl(url: string): Promise<boolean> {
  const normalizedUrl = url.trim();
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
