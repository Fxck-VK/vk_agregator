import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const consumers = [
  ["src/components/ui/ModalBackdrop/ModalBackdrop.tsx", "src/components/ui/ModalBackdrop/ModalBackdrop.module.css"],
  ["src/components/chat/ChatFilePicker/ChatFilePicker.tsx", "src/components/chat/ChatFilePicker/ChatFilePicker.module.css"],
  ["src/components/layout/AppShell/AppShell.tsx", "src/components/layout/AppShell/AppShell.module.css"],
  [
    "src/components/layout/WorkspaceHeader/SubscriptionPlansDialog.tsx",
    "src/components/layout/WorkspaceHeader/SubscriptionPlansDialog.module.css",
  ],
  [
    "src/features/conversations/SidebarConversations/SidebarConversations.tsx",
    "src/features/conversations/SidebarConversations/SidebarConversations.module.css",
  ],
  ["src/components/ui/PopoverPanel/PopoverPanel.tsx", "src/components/ui/PopoverPanel/PopoverPanel.module.css"],
  [
    "src/features/account/AccountUpdatesPanel/AccountUpdatesPanel.tsx",
    "src/features/account/AccountUpdatesPanel/AccountUpdatesPanel.module.css",
  ],
  [
    "src/features/models/WorkspaceModelSelector/ModelSelector.tsx",
    "src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.module.css",
  ],
  [
    "src/features/image-generation/ImageTemplatePicker/ImageTemplatePicker.tsx",
    "src/features/image-generation/ImageTemplatePicker/ImageTemplatePicker.module.css",
  ],
  [
    "src/features/image-generation/ImageQualitySelector/ImageQualitySelector.tsx",
    "src/features/image-generation/ImageQualitySelector/ImageQualitySelector.module.css",
  ],
  [
    "src/features/image-generation/ImageAspectRatioSelector/ImageAspectRatioSelector.tsx",
    "src/features/image-generation/ImageAspectRatioSelector/ImageAspectRatioSelector.module.css",
  ],
  [
    "src/components/media/MediaPreviewDialogTemplate/MediaPreviewDialogTemplate.tsx",
    "src/components/media/MediaPreviewDialogTemplate/MediaPreviewDialogTemplate.module.css",
  ],
  [
    "src/features/models/ModelCatalogToolbar/ModelCatalogToolbar.tsx",
    "src/features/models/ModelCatalogToolbar/ModelCatalogToolbar.module.css",
  ],
  [
    "src/features/files/FileTypeTabs/FileTypeTabs.tsx",
    "src/features/files/FileTypeTabs/FileTypeTabs.module.css",
  ],
  [
    "src/features/image-generation/ImageGenerationGuide/ImageGenerationGuide.tsx",
    "src/features/image-generation/ImageGenerationGuide/ImageGenerationGuide.module.css",
  ],
  [
    "src/features/account/ProfileWorkspace/ProfileWorkspace.tsx",
    "src/features/account/ProfileWorkspace/ProfileWorkspace.module.css",
  ],
  [
    "src/components/public/PublicHeader/PublicHeader.tsx",
    "src/components/public/PublicHeader/PublicHeader.module.css",
  ],
  [
    "src/components/chat/AssistantMessageContent/AssistantMessageContent.tsx",
    "src/components/chat/AssistantMessageContent/AssistantMessageContent.module.css",
  ],
] as const;

describe("ScrollArea consumer contract", () => {
  it.each(consumers)("uses the shared floating scrollbar in %s", (sourcePath) => {
    const source = readFileSync(resolve(process.cwd(), sourcePath), "utf8");

    expect(source).toContain('from "@/components/ui/ScrollArea/ScrollArea"');
    expect(source).toContain("<ScrollArea");
  });

  it.each(consumers)("removes local scrollbar visuals from %s", (_sourcePath, stylesheetPath) => {
    const stylesheet = readFileSync(resolve(process.cwd(), stylesheetPath), "utf8");

    expect(stylesheet).not.toMatch(/overflow:\s*(?:auto|scroll)/);
    expect(stylesheet).not.toMatch(/overflow-x:\s*(?:auto|scroll)/);
    expect(stylesheet).not.toMatch(/overflow-y:\s*(?:auto|scroll)/);
    expect(stylesheet).not.toContain("scrollbar-color:");
    expect(stylesheet).not.toContain("scrollbar-width:");
    expect(stylesheet).not.toContain("::-webkit-scrollbar");
  });
});
