import { act } from "react";
import { createRoot } from "react-dom/client";
import { describe, expect, it } from "vitest";

import { capabilityRows, ModelCapabilitiesDetails } from "./ModelCapabilitiesDetails";
import type { CapabilityProfile, InputCapability, ModelCapabilities } from "./model-capabilities";

const unsupportedInput: InputCapability = { support: "unsupported", extensions: [], max_count: 0 };
const unknownInput: InputCapability = { support: "unknown", extensions: [], max_count: null };
const supportedInput: InputCapability = { support: "supported", extensions: null, max_count: null };

const mixedTextCapabilities: ModelCapabilities = {
  schema_version: 1,
  api: {
    text: {
      images: supportedInput,
      videos: unknownInput,
      files: supportedInput,
    },
    notes: ["Native API проверен отдельно."],
  },
  application: {
    text: {
      images: unsupportedInput,
      videos: unsupportedInput,
      files: unsupportedInput,
    },
    notes: ["В этом интерфейсе доступны только текстовые prompts."],
  },
};

function rowValue(rows: [string, string][], label: string) {
  const row = rows.find(([name]) => name === label);
  if (!row) throw new Error(`Missing row ${label}`);
  return row[1];
}

function sectionText(container: HTMLElement, label: string) {
  const section = Array.from(container.querySelectorAll("section")).find((node) => node.getAttribute("aria-label") === label);
  if (!section) throw new Error(`Missing section ${label}`);
  return section.textContent ?? "";
}

async function renderDetails(capabilities: ModelCapabilities) {
  Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true });
  const container = document.createElement("div");
  document.body.append(container);
  const root = createRoot(container);
  await act(async () => {
    root.render(<ModelCapabilitiesDetails capabilities={capabilities} />);
  });
  return {
    container,
    async cleanup() {
      await act(async () => {
        root.unmount();
      });
      container.remove();
    },
  };
}

describe("ModelCapabilitiesDetails", () => {
  it("renders application support separately from native API support", async () => {
    const view = await renderDetails(mixedTextCapabilities);

    try {
      const app = sectionText(view.container, "В этом интерфейсе");
      const api = sectionText(view.container, "В API провайдера");

      expect(app.match(/Нет/g) ?? []).toHaveLength(3);
      expect(app).not.toContain("Не подтверждено");
      expect(api).toContain("Не подтверждено");
      expect(api.match(/Да, максимум не подтверждён/g) ?? []).toHaveLength(2);
      expect(api).not.toContain("Нет");
    } finally {
      await view.cleanup();
    }
  });

  it("does not collapse unknown input support into no support", () => {
    expect(rowValue(capabilityRows({ text: { images: unknownInput, videos: unsupportedInput, files: supportedInput } }), "Фото")).toBe("Не подтверждено");
    expect(rowValue(capabilityRows({ text: { images: unsupportedInput, videos: unknownInput, files: supportedInput } }), "Фото")).toBe("Нет");
  });

  it("keeps image speed modes distinct from image resolutions", () => {
    const rows = capabilityRows({
      image: {
        images: unsupportedInput,
        aspect_ratios: ["1:1"],
        resolutions: ["1024x1024"],
        quality_modes: ["high"],
        speed_modes: ["fast"],
        max_output_count: 1,
        max_combined_images: null,
      },
    });

    expect(rowValue(rows, "Разрешение")).toBe("1024x1024");
    expect(rowValue(rows, "Скорость")).toBe("fast");
  });

  it("renders video min/max duration, audio mode, and end-frame metadata", () => {
    const profile: CapabilityProfile = {
      video: {
        images: { support: "supported", extensions: [".jpg"], max_count: 2 },
        videos: { support: "supported", extensions: [".mp4"], max_count: 1 },
        allowed_image_counts: [0, 1, 2],
        duration: {
          mode: "reference_video",
          min_seconds: 3,
          max_seconds: 30,
          allowed_seconds: [5, 10],
          by_resolution: { "720p": [5, 10] },
          max_by_orientation: { portrait: 15 },
        },
        resolutions: ["720p"],
        quality_modes: ["std"],
        aspect_ratios: ["16:9"],
        audio: { mode: "preserve_source", selectable: true },
        start_frame: "required",
        end_frame: "optional",
      },
    };
    const rows = capabilityRows(profile);

    expect(rowValue(rows, "Длительность")).toBe("По исходному видео. 3–30 с");
    expect(rowValue(rows, "Доступные длительности, с")).toBe("5, 10");
    expect(rowValue(rows, "Длительность для 720p, с")).toBe("5, 10");
    expect(rowValue(rows, "Максимум при ориентации portrait")).toBe("15 с");
    expect(rowValue(rows, "Звук")).toBe("Можно сохранить звук исходного видео");
    expect(rowValue(rows, "Начальный кадр")).toBe("Обязательно");
    expect(rowValue(rows, "Конечный кадр")).toBe("Необязательно");
  });
});
