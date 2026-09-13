import type { ImageModel } from "@/lib/web-api/contracts";

export const catalogPlaceholderModels = [
  {
    id: "catalog-preview-recraft-v3",
    name: "Recraft V3",
    quality_options: ["1K", "2K"],
    price_by_quality: { "1K": 35, "2K": 55 },
    default_quality: "1K",
    supports_reference_image: false,
    max_reference_images: 0,
    max_output_count: 4,
  },
  {
    id: "catalog-preview-ideogram-3",
    name: "Ideogram 3",
    quality_options: ["1K", "2K"],
    price_by_quality: { "1K": 38, "2K": 58 },
    default_quality: "1K",
    supports_reference_image: true,
    max_reference_images: 2,
    max_output_count: 4,
  },
  {
    id: "catalog-preview-stable-diffusion-3-5",
    name: "Stable Diffusion 3.5",
    quality_options: ["1K", "2K"],
    price_by_quality: { "1K": 25, "2K": 45 },
    default_quality: "1K",
    supports_reference_image: true,
    max_reference_images: 2,
    max_output_count: 4,
  },
  {
    id: "catalog-preview-leonardo-phoenix",
    name: "Leonardo Phoenix",
    quality_options: ["1K", "2K"],
    price_by_quality: { "1K": 32, "2K": 52 },
    default_quality: "1K",
    supports_reference_image: true,
    max_reference_images: 2,
    max_output_count: 4,
  },
] as const satisfies readonly ImageModel[];

const catalogPlaceholderIDs = new Set(catalogPlaceholderModels.map(({ id }) => id));

export function isCatalogPlaceholderModel(model: Pick<ImageModel, "id">): boolean {
  return catalogPlaceholderIDs.has(model.id as (typeof catalogPlaceholderModels)[number]["id"]);
}
