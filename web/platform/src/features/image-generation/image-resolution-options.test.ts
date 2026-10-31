import {describe,expect,it} from "vitest";
import {imageResolutionLabel,imageResolutionOptions} from "./image-resolution-options";

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
  it("shows 8K only when supplied and sorts supported sizes numerically",()=>{
    const options=["8K-pro","2K-pro","1K-pro","4K-pro","4K-pro"];
    expect(imageResolutionOptions(options,"2K-pro")).toEqual(["1K-pro","2K-pro","4K-pro","8K-pro"]);
    expect(imageResolutionOptions(["1K","2K"],"1K")).toEqual(["1K","2K"]);
    expect(imageResolutionOptions([],"1K")).toEqual([]);
  });
  it("does not turn non-resolution modes into invented K sizes",()=>{
    expect(imageResolutionOptions(["standard","hd","standard"],"hd")).toEqual(["standard","hd"]);
    expect(imageResolutionLabel("standard")).toBe("Стандарт");
    expect(imageResolutionLabel("hd")).toBe("hd");
  });
});
