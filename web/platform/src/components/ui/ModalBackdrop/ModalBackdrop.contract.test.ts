import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

describe("ModalBackdrop contract", () => {
  it("provides one shared backdrop component for modal windows", () => {
    expect(
      existsSync(resolve(process.cwd(), "src/components/ui/ModalBackdrop/ModalBackdrop.tsx")),
    ).toBe(true);
  });

  const modalConsumers = [
    "src/components/layout/WorkspaceHeader/SubscriptionPlansDialog.tsx",
    "src/components/media/MediaPreviewDialogTemplate/MediaPreviewDialogTemplate.tsx",
    "src/features/image-generation/ImageTemplatePicker/ImageTemplatePicker.tsx",
    "src/components/chat/ChatFilePicker/ChatFilePicker.tsx",
    "src/features/conversations/ConversationRow/ConversationDeleteDialog.tsx",
  ];

  it.each(modalConsumers)("uses the shared backdrop in %s", (consumerPath) => {
    const source = readFileSync(resolve(process.cwd(), consumerPath), "utf8");

    expect(source).toContain('from "@/components/ui/ModalBackdrop/ModalBackdrop"');
    expect(source).toContain("<ModalBackdrop");
  });
});
