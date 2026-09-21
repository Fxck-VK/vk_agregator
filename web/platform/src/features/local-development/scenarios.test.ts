import { describe, expect, it } from "vitest";
import { defaultScenarios, parseScenarios } from "./scenarios";

describe("local scenario preferences", () => {
  it.each([null, "invalid", "{}", '{"enabled":true,"upload":"unknown","quote":"success"}'])("recovers safely from %s", value => {
    expect(parseScenarios(value)).toEqual(defaultScenarios);
  });
  it("restores only validated scenario fields", () => {
    expect(parseScenarios('{"enabled":false,"upload":"slow","quote":"error","untrusted":"ignored"}')).toEqual({ enabled: false, upload: "slow", quote: "error", message: "success" });
  });
  it("restores offline uploads without enabling an unsupported message scenario", () => {
    expect(parseScenarios('{"enabled":true,"upload":"offline","quote":"success","message":"offline"}')).toEqual({ ...defaultScenarios, upload: "offline" });
  });
});
