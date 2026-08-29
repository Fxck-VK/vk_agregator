import "server-only";

import type {
  AccountProfile,
  ConversationItem,
  ImageModelList,
} from "../../lib/web-api/contracts";

export const localWorkspacePreviewProfile: AccountProfile = {
  account_id: "10000000-0000-4000-8000-000000000001",
  identity_refs: [
    {
      id: "10000000-0000-4000-8000-000000000002",
      account_id: "10000000-0000-4000-8000-000000000001",
      provider: "email",
      label: "preview@neirohub.local",
      verified: true,
      created_at: "2026-01-01T00:00:00Z",
    },
  ],
};

export const localWorkspacePreviewBalance = 1000;

export const localWorkspacePreviewConversations: ConversationItem[] = [
  {
    id: "20000000-0000-4000-8000-000000000001",
    title: "Подготовить макет",
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:05:00Z",
  },
  {
    id: "20000000-0000-4000-8000-000000000002",
    title: "Идеи для проекта",
    created_at: "2026-01-02T00:00:00Z",
    updated_at: "2026-01-02T00:05:00Z",
  },
  {
    id: "20000000-0000-4000-8000-000000000003",
    title: "Тексты для сайта",
    created_at: "2026-01-03T00:00:00Z",
    updated_at: "2026-01-03T00:05:00Z",
  },
];

export const localWorkspacePreviewImageModels: ImageModelList = {
  items: [
    {
      id: "nano-banana-2",
      name: "Nano Banana 2",
      quality_options: ["1K", "2K", "4K"],
      price_by_quality: { "1K": 50, "2K": 70, "4K": 90 },
      default_quality: "1K",
      supports_reference_image: true,
      max_reference_images: 4,
      max_output_count: 4,
    },
    {
      id: "nano-banana-pro",
      name: "Nano Banana Pro",
      quality_options: ["1K", "2K", "4K"],
      price_by_quality: { "1K": 50, "2K": 80, "4K": 110 },
      default_quality: "1K",
      supports_reference_image: true,
      max_reference_images: 4,
      max_output_count: 4,
    },
    {
      id: "gpt-image-2",
      name: "GPT Image 2",
      quality_options: ["1K", "2K", "4K"],
      price_by_quality: { "1K": 40, "2K": 65, "4K": 90 },
      default_quality: "1K",
      supports_reference_image: true,
      max_reference_images: 4,
      max_output_count: 4,
    },
    {
      id: "seedream-4-5",
      name: "Seedream 4.5",
      quality_options: ["2K", "4K"],
      price_by_quality: { "2K": 30, "4K": 50 },
      default_quality: "2K",
      supports_reference_image: true,
      max_reference_images: 4,
      max_output_count: 4,
    },
  ],
};

export function isLocalWorkspacePreviewEnabled(): boolean {
  return (
    process.env.NODE_ENV === "development" &&
    process.env.NEIROHUB_LOCAL_WORKSPACE_PREVIEW === "1"
  );
}
