import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

import catalog from "@/features/session/model-catalog.preview.json";
import { brandLogoPath } from "@/config/brand";
import { getModelPresentation } from "./ModelCard/model-card-content";
import { getModelIconPath, modelIconRegistry } from "./model-icon-registry";

describe("model icon registry", () => {
  it("covers the full catalog without guessing icons for unlisted models", () => {
    expect(Object.keys(modelIconRegistry).sort()).toEqual(catalog.items.map(model => model.id).sort());
    for (const model of catalog.items) {
      const path = getModelIconPath(model.id);
      expect(path === brandLogoPath || /^\/assets\/images\/models\/.*\.svg$/.test(path ?? ""), model.id).toBe(true);
      expect(getModelPresentation(model).artworkSrc, model.id).toBe(path);
    }
  });

  it("uses the same mark for related versions and the developer mark where needed", () => {
    for (const family of [
      ["gpt_image_2", "gpt_6_astra", "gpt_4o_mini_tts", "whisper_1"],
      ["seedream_4_5", "seedream_5_0_pro", "seedance_2_0", "video_seedance_2_5"],
      ["flux_2_pro", "flux_2_flex", "flux_3_video"],
      ["nano_banana", "nano_banana_2", "imagen_4_0", "video_veo_3_1_fast", "lyria_3_5"],
      ["grok_image_2_0", "grok_imagine_1_5_video", "grok_4_7"],
      ["claude_opus_4_6", "claude_sonnet_4_6", "claude_fable_5", "claude_haiku_4_5_20251001"],
      ["gemini_2_5_pro", "gemini_3_7_flash", "gemini_omni_flash_preview", "video_gemini_omni_1_1_flash_ext"],
      ["qwen_image_3", "qwen_3_8_max"],
      ["video_kling_v3", "kling_video_o1", "kling_2_6"],
      ["video_minimax_h3", "minimax_h3_max"],
      ["wan_2_7", "wan_3_0", "z_image_turbo"],
      ["suno_v6", "suno_v6_wild", "suno_v6_mini"],
      ["deepseek_v4_pro", "deepseek_r1_250528"],
      ["vidu_q3_pro", "vidu_q3_mix", "vidu_q3_turbo"],
      ["skyreels_v4_fast", "skyreels_v4_std"],
      ["happyhorse_1_0", "happyhorse_1_1"],
    ]) {
      expect(getModelIconPath(family[0])).toBeDefined();
      expect(new Set(family.map(getModelIconPath)).size, family.join(", ")).toBe(1);
    }
    // This ID is the site's branded chat, not the ChatGPT product.
    expect(getModelIconPath("chatgpt")).toBe(brandLogoPath);
    expect(getModelIconPath("chatgpt")).not.toBe(getModelIconPath("gpt_5"));
  });

  it("provides square, self-contained vector artwork with room around each mark", () => {
    for (const path of new Set(Object.values(modelIconRegistry))) {
      const file = readFileSync(resolve(process.cwd(), "public", path.slice(1)));
      if (path === brandLogoPath) {
        expect(file.byteLength).toBeGreaterThan(0);
        continue;
      }
      if (path.endsWith(".png")) {
        expect(file.subarray(0, 8).toString("hex")).toBe("89504e470d0a1a0a");
        continue;
      }
      const xml = file.toString("utf8");
      const document = new DOMParser().parseFromString(xml, "image/svg+xml");
      expect(document.querySelector("parsererror")).toBeNull();
      expect(document.documentElement.getAttribute("viewBox")).toBe("-4 -4 32 32");
      expect(document.querySelector("path")).not.toBeNull();
      expect(document.querySelector("script, image, foreignObject, use")).toBeNull();
      expect(document.querySelector("[href], [style], [onload]")).toBeNull();
    }
  });

  it("preserves supplied artwork and the fallback for unknown models", () => {
    expect(getModelPresentation({ id: "custom_model", name: "Custom", artworkSrc: "/assets/custom.svg" }).artworkSrc).toBe("/assets/custom.svg");
    for (const id of ["gpt_future", "seedream_future", "constructor", "__proto__", "unknown-model"]) {
      expect(getModelIconPath(id)).toBeUndefined();
      expect(getModelPresentation({ id, name: "Unknown" }).artworkSrc).toBeUndefined();
    }
  });
});
