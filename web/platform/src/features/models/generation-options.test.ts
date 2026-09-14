import { expect, it } from "vitest";
import { resolveGenerationOptions } from "./generation-options";
import type { GenerationModel } from "./generation-model-catalog";

it("uses the server image default and price for the selected combination", () => {
  const image = {
    id:"image", name:"Image", category:"images", quality_options:["1K"], default_quality:"1K",
    allowed_aspect_ratios:["16:9","1:1"], default_aspect_ratio:"1:1", max_output_count:4,
    supports_reference_image:false,max_reference_images:0,price_by_quality:{"1K":10},
    price_by_variant:{"1K:1:1":10,"1K:16:9":20},
  } as GenerationModel;
  expect(resolveGenerationOptions(image).options.aspect_ratio).toBe("1:1");
  expect(resolveGenerationOptions(image,{aspect_ratio:"16:9",output_count:2}).cost).toBe(40);
});

it("repairs a saved video combination when duration is unavailable at its resolution", () => {
  const video = {
    id:"video",name:"Video",category:"video",description:"Video",
    allowed_resolutions:["720p","1080p"],allowed_durations_sec:[5,10],allowed_aspect_ratios:["16:9","9:16"],
    default_resolution:"720p",default_duration_sec:10,default_aspect_ratio:"16:9",
    price_by_option:{"720p:5":50,"720p:10":100,"1080p:5":100},
    variants:[
      {resolution:"720p",duration_sec:5,aspect_ratio:"16:9",fps:0,audio:false},
      {resolution:"720p",duration_sec:10,aspect_ratio:"16:9",fps:0,audio:false},
      {resolution:"1080p",duration_sec:5,aspect_ratio:"9:16",fps:0,audio:false},
    ],
  } as GenerationModel;
  expect(resolveGenerationOptions(video,{resolution:"1080p",duration_sec:10,aspect_ratio:"16:9"})).toEqual({options:{resolution:"1080p",duration_sec:5,aspect_ratio:"9:16"},cost:100});
});
