export type UploadScenario = "success" | "error" | "slow";
export type LocalScenarios = { enabled: boolean; upload: UploadScenario | "offline"; quote: "success" | "error"; message: UploadScenario };
export const scenarioStorageKey = "neirohub.local-scenarios.v1";
export const defaultScenarios: LocalScenarios = { enabled: true, upload: "success", quote: "success", message: "success" };

export function parseScenarios(value: string | null): LocalScenarios {
  try {
    const parsed = JSON.parse(value ?? "null");
    if (parsed && typeof parsed.enabled === "boolean" && ["success", "error", "slow", "offline"].includes(parsed.upload) && ["success", "error"].includes(parsed.quote)) {
      return { enabled: parsed.enabled, upload: parsed.upload, quote: parsed.quote, message: ["success", "error", "slow"].includes(parsed.message) ? parsed.message : "success" };
    }
  } catch { /* Invalid preferences must not prevent the workspace from opening. */ }
  return { ...defaultScenarios };
}
