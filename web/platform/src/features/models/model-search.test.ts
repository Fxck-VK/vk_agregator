import { describe, expect, it } from "vitest";

import { searchModels } from "./model-search";

const models = [
  { id: "nano_banana_2", name: "Nano Banana 2" },
  { id: "nano_banana_pro", name: "Nano Banana Pro" },
  { id: "seedream_5_pro", name: "Seedream 5.0 Pro" },
  { id: "video_kling_v3", name: "Kling V3" },
  { id: "gpt_5_5", name: "GPT-5.5" },
  { id: "gpt_5_6", name: "GPT-5.6" },
  { id: "claude_opus", name: "Claude Opus" },
  { id: "gemini_pro", name: "Gemini Pro" },
];

describe("searchModels", () => {
  it.each([
    ["нано банана про", "nano_banana_pro"],
    ["нана банана про", "nano_banana_pro"],
    ["  НАНОБАНАНА про  ", "nano_banana_pro"],
    ["банана про", "nano_banana_pro"],
    ["nanobanana pro", "nano_banana_pro"],
    ["seedrem", "seedream_5_pro"],
    ["сидрим", "seedream_5_pro"],
    ["клинг", "video_kling_v3"],
    ["клод опус", "claude_opus"],
    ["джемини про", "gemini_pro"],
    ["пзе", "gpt_5_5"],
    ["тфтщ ифтфтф зкщ", "nano_banana_pro"],
    ["gpt5.5", "gpt_5_5"],
    ["nano_banana_pro", "nano_banana_pro"],
  ])("finds %s and ranks the intended model first", (query, id) => {
    expect(searchModels(models, query)[0]?.id).toBe(id);
  });

  it("accepts a one-character typo and adjacent transposition", () => {
    expect(searchModels(models, "nana bananna pro")).toEqual([models[1]]);
    expect(searchModels(models, "bannaa pro")).toEqual([models[1]]);
  });

  it("puts exact names before partial names and typos with stable ties", () => {
    const candidates = [
      { id: "typo", name: "Bananna Pro" },
      models[1],
      { id: "exact", name: "Banana Pro" },
      { id: "another", name: "Other Banana Pro" },
    ];
    expect(searchModels(candidates, "banana pro").map(model => model.id)).toEqual([
      "exact", "nano_banana_pro", "another", "typo",
    ]);
    expect(candidates[0].id).toBe("typo");
  });

  it("keeps version numbers exact and does not match a missing qualifier", () => {
    expect(searchModels(models, "gpt 5.5")).toEqual([models[4]]);
    expect(searchModels(models, "nano banana pro")).toEqual([models[1]]);
    expect(searchModels(models, "nano banana ultra")).toEqual([]);
    expect(searchModels(models, "gpt 5.9")).toEqual([]);
    expect(searchModels(models, "gpt 6.5")).toEqual([]);
    expect(searchModels(models, "gpt 55")).toEqual([]);
  });

  it("never invents results outside the supplied available set", () => {
    expect(searchModels(models.slice(0, 1), "нано банана про")).toEqual([]);
    expect(searchModels(models, "совсем неизвестная нейросеть")).toEqual([]);
    expect(searchModels(models, "!?")).toEqual([]);
    expect(searchModels(models, "a".repeat(1000))).toEqual([]);
  });

  it("preserves the original objects and order for a blank query", () => {
    const result = searchModels(models, " \t ");
    expect(result).toEqual(models);
    expect(result[0]).toBe(models[0]);
  });
});
