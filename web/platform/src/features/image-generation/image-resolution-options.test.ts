import {describe,expect,it} from "vitest";
import {imageResolutionLabel,imageResolutionOptions,resolveImageResolution} from "./image-resolution-options";

describe("size-only resolution choices",()=>{
  it("collapses quality tiers into actual size choices while preserving the active backend tier",()=>{
    const options=["1K-low","1K-medium","1K-high","2K-low","2K-medium","2K-high","4K-low","4K-medium","4K-high"];
    const result=imageResolutionOptions(options,"1K-medium");
    expect(result).toEqual(["1K-medium","2K-medium","4K-medium"]);
    expect(result.map(value=>imageResolutionLabel(value))).toEqual(["1K","2K","4K"]);
    expect(imageResolutionOptions(options,"4K-high")).toEqual(["1K-high","2K-high","4K-high"]);
  });
  it("keeps an exact active code and uses only catalogue entries when a matching tier is unavailable",()=>{
    const options=["4K-low","2K","1K-ultra-pro","1K-low"];
    expect(imageResolutionOptions(options,"1K-ultra-pro")).toEqual(["1K-ultra-pro","2K","4K-low"]);
    expect(imageResolutionLabel("1K-ultra-pro")).toBe("1K");
  });
  it("limits K choices to the supported 1K, 2K and 4K sizes in ascending order",()=>{
    const options=["8K-pro","2K-pro","1K-pro","4K-pro","4K-pro"];
    expect(imageResolutionOptions(options,"2K-pro")).toEqual(["1K-pro","2K-pro","4K-pro"]);
    expect(imageResolutionOptions(["1K","2K"],"1K")).toEqual(["1K","2K"]);
    expect(imageResolutionOptions([],"1K")).toEqual([]);
  });
  it("hides fractional and other K sizes while retaining real tier codes",()=>{
    const options=["4K-high","1.5K-high","3K-high","2K-high","2.5K-high","1K-high","8K-high"];
    expect(imageResolutionOptions(options,"1.5K-high")).toEqual(["1K-high","2K-high","4K-high"]);
    expect(imageResolutionOptions(["1.5K"],"1.5K")).toEqual([]);
  });
  it("does not turn non-resolution modes into invented K sizes",()=>{
    expect(imageResolutionOptions(["standard","hd","standard"],"hd")).toEqual(["standard","hd"]);
    expect(imageResolutionLabel("standard")).toBe("Стандарт");
    expect(imageResolutionLabel("hd")).toBe("hd");
  });
  it("resolves hidden sizes to the nearest admitted size without losing a matching tier",()=>{
    expect(resolveImageResolution(["1K-high","2K-high","2.5K-high","4K-high"],"2.5K-high")).toBe("2K-high");
    expect(resolveImageResolution(["1K","1.5K","2K"],"1.5K","2K")).toBe("2K");
    expect(resolveImageResolution(["1.5K"],"1.5K")).toBe("");
    expect(resolveImageResolution(["standard","hd"],"hd","standard")).toBe("hd");
  });
});
