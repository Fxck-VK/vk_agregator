import { describe, expect, it } from "vitest";

import { assetPaths } from "./asset-paths";

describe("assetPaths", () => {
  it("exposes stable account-menu icon URLs", () => {
    expect(assetPaths.icons.accountMenu).toEqual({
      logout: "/assets/icons/account-menu/logout.svg",
      megaphone: "/assets/icons/account-menu/megaphone.svg",
      profile: "/assets/icons/account-menu/profile.svg",
      support: "/assets/icons/account-menu/support.svg",
    });
  });

  it("exposes stable theme icon URLs", () => {
    expect(assetPaths.icons.theme).toEqual({
      monitor: "/assets/icons/theme/monitor.svg",
      moon: "/assets/icons/theme/moon.svg",
      sun: "/assets/icons/theme/sun.svg",
    });
  });

  it("exposes a stable inspiration image URL without eager imports", () => {
    expect(assetPaths.images.inspiration.paperCraneCloud).toBe(
      "/assets/images/inspiration/paper-crane-cloud.png",
    );
  });
});
