import { describe, expect, test } from "vitest";

import {
  artifactUrl,
  errorLabel,
  launchParamsFromLocation,
  normalizeRawParams,
  referralCodeFromRaw,
  statusKind,
  stringifyBridgeLaunchParams,
  telemetryLabel,
  telemetryRoute,
} from "./client";

const ARTIFACT_ID = "550e8400-e29b-41d4-a716-446655440000";

describe("telemetry safety helpers", () => {
  test("normalizes routes without query, hash, prompts or launch params", () => {
    const route = telemetryRoute(
      `/miniapp/jobs/${ARTIFACT_ID}?prompt=secret-text&launch_params=vk_user_id%3D1#private_url=secret`,
    );

    expect(route).toBe("/miniapp/jobs/:id");
    expect(route).not.toContain("prompt");
    expect(route).not.toContain("launch");
    expect(route).not.toContain("secret");
    expect(route).not.toContain(ARTIFACT_ID);
  });

  test("bounds label characters and length", () => {
    const label = telemetryLabel(" Payment Failed!!! /Raw URL?x=1 ".repeat(4), "unknown");

    expect(label.length).toBeLessThanOrEqual(96);
    expect(label).toMatch(/^[a-z0-9_./:-]+$/);
    expect(label).not.toContain("?");
  });
});

describe("launch and referral parsing helpers", () => {
  test("normalizes raw query/hash prefixes", () => {
    expect(normalizeRawParams("?vk_user_id=42")).toBe("vk_user_id=42");
    expect(normalizeRawParams("#vk_user_id=42")).toBe("vk_user_id=42");
  });

  test("extracts launch params only when a VK user identity is present", () => {
    window.history.replaceState({}, "", "/?vk_user_id=42&vk_ts=1&sign=fake");
    expect(launchParamsFromLocation()).toBe("vk_user_id=42&vk_ts=1&sign=fake");

    window.history.replaceState({}, "", "/?ref=ABCD1234");
    expect(launchParamsFromLocation()).toBe("");
  });

  test("serializes bridge launch params without undefined or null values", () => {
    const raw = stringifyBridgeLaunchParams({
      vk_user_id: 42,
      vk_ts: 1,
      sign: "fake",
      ignored: undefined,
      empty: null,
    });

    expect(raw).toContain("vk_user_id=42");
    expect(raw).toContain("vk_ts=1");
    expect(raw).toContain("sign=fake");
    expect(raw).not.toContain("ignored");
    expect(raw).not.toContain("empty");
  });

  test("accepts only public referral-code shape", () => {
    expect(referralCodeFromRaw("?ref=ABCD2345")).toBe("ABCD2345");
    expect(referralCodeFromRaw("?ref=bad code")).toBe("");
    expect(referralCodeFromRaw("?ref=vk_user_id=42")).toBe("");
  });
});

describe("artifact and status helpers", () => {
  test("builds artifact URLs from UUIDs only", () => {
    expect(artifactUrl(ARTIFACT_ID)).toBe(`/miniapp/artifacts/${ARTIFACT_ID}`);
    expect(artifactUrl(`${ARTIFACT_ID}?launch_params=secret`)).toBeNull();
    expect(artifactUrl("https://storage.local/private/file.png")).toBeNull();
  });

  test("maps terminal statuses without exposing backend details", () => {
    expect(statusKind("succeeded")).toBe("done");
    expect(statusKind("failed_terminal")).toBe("failed");
    expect(statusKind("provider_running")).toBe("progress");
    expect(errorLabel({ status: "failed_terminal", error_code: "unknown" } as never)).toBeTruthy();
  });
});
