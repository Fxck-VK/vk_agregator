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

  it("exposes the shared model-selector chevron URL", () => {
    expect(assetPaths.icons.ui.chevronDown).toBe(
      "/assets/icons/ui/chevron-down.svg",
    );
  });

  it("exposes the shared media-upload icon URL", () => {
    expect(assetPaths.icons.ui.uploadMedia).toBe(
      "/assets/icons/ui/upload-media.svg",
    );
  });

  it("exposes the shared send-message icon URL", () => {
    expect(assetPaths.icons.ui.sendMessage).toBe(
      "/assets/icons/ui/send-message-white.svg",
    );
  });

  it("exposes the image-composer control icon URLs", () => {
    expect(assetPaths.icons.ui.templateSelect).toBe(
      "/assets/icons/ui/template-select-white.svg",
    );
    expect(assetPaths.icons.ui.resolution).toBe(
      "/assets/icons/ui/resolution-white.svg",
    );
  });

  it("exposes a stable inspiration image URL without eager imports", () => {
    expect(assetPaths.images.inspiration.paperCraneCloud).toBe(
      "/assets/images/inspiration/paper-crane-cloud.png",
    );
  });

  it("exposes the files empty-state illustration URL", () => {
    expect(assetPaths.illustrations.filesEmptyFolder).toBe(
      "/assets/illustrations/files-empty-folder.png",
    );
  });

  it("exposes the shared credit-star image URL", () => {
    expect(assetPaths.images.credits.star).toBe(
      "/assets/images/credits/credit-star.png",
    );
  });

  it("exposes the all-models count badge URL", () => {
    expect(assetPaths.images.workspace.allModelsBadge).toBe(
      "/assets/images/workspace/all-models-90-plus.png",
    );
  });

  it("exposes the all-models button artwork URL", () => {
    expect(assetPaths.images.workspace.allModelsButtonBackground).toBe(
      "/assets/images/workspace/all-models-button-background.png",
    );
  });

  it("exposes the workspace how-to poster URL", () => {
    expect(assetPaths.images.workspace.howItWorksPoster).toBe(
      "/assets/images/workspace/neirohub-how-it-works-poster.png",
    );
  });

  it("exposes the workspace how-to video URL", () => {
    expect(assetPaths.videos.workspace.howItWorks).toBe(
      "/assets/videos/workspace/neirohub-how-it-works.mp4",
    );
  });

  it("exposes both theme-specific model placeholder URLs", () => {
    expect(assetPaths.images.models.fallback).toEqual({
      darkTheme: "/assets/images/models/chip-silhouette.svg",
      lightTheme: "/assets/images/models/chip-silhouette-dark.svg",
    });
  });
});
