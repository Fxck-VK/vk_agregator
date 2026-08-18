import { webBrowserMutation } from "@/lib/web-api/browser";

const attemptTimeoutMs = 1_300;
const retryDelaysMs = [250, 500] as const;

class WorkspaceLogoutResponseError extends Error {}

function wait(delayMs: number): Promise<void> {
  return new Promise((resolve) => window.setTimeout(resolve, delayMs));
}

async function requestAttempt(): Promise<void> {
  const controller = new AbortController();
  const timeout = window.setTimeout(() => controller.abort(), attemptTimeoutMs);

  try {
    const response = await webBrowserMutation("/web/v1/auth/logout", {
      method: "POST",
      signal: controller.signal,
    });
    if (response.status !== 204) {
      throw new WorkspaceLogoutResponseError();
    }
  } finally {
    window.clearTimeout(timeout);
  }
}

export async function requestWorkspaceLogout(): Promise<void> {
  for (let attempt = 0; attempt <= retryDelaysMs.length; attempt += 1) {
    try {
      await requestAttempt();
      return;
    } catch (error) {
      const isTerminal = error instanceof WorkspaceLogoutResponseError || attempt === retryDelaysMs.length;
      if (isTerminal) {
        throw new Error("Unable to complete workspace logout.");
      }
      await wait(retryDelaysMs[attempt]);
    }
  }
}

