import { render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

vi.mock("next/navigation", () => ({
  useRouter: vi.fn(() => ({ push: vi.fn() })),
}));

vi.mock("@/features/models/image-model-catalog-cache", () => ({
  loadImageModelCatalog: vi.fn().mockResolvedValue({ items: [] }),
}));

import { WorkspaceConversationListProvider } from "@/features/conversations/WorkspaceConversationList/WorkspaceConversationList";

import { WorkspaceLanding } from "./WorkspaceLanding";
import { capabilityLinks } from "./workspace-home-content";

describe("WorkspaceLanding capability links", () => {
  it("renders the approved divider and six icon links without changing their destinations", () => {
    render(
      <WorkspaceConversationListProvider accountId="capability-links-test-account" initialConversations={[]}>
        <WorkspaceLanding />
      </WorkspaceConversationListProvider>,
    );

    expect(screen.getByRole("heading", { level: 3, name: "И многое другое" })).toBeInTheDocument();

    const navigation = screen.getByRole("navigation", { name: "Дополнительные возможности" });
    const links = within(navigation).getAllByTestId("workspace-capability-link");

    expect(links).toHaveLength(6);
    expect(links.map((link) => link.textContent)).toEqual(capabilityLinks.map((item) => item.label));
    expect(links.map((link) => link.getAttribute("href"))).toEqual(capabilityLinks.map((item) => item.href));

    const fallbackIcons = within(navigation).getAllByTestId("workspace-capability-icon");

    expect(fallbackIcons).toHaveLength(6);
    for (const icon of fallbackIcons) {
      expect(icon).toHaveAttribute("aria-hidden", "true");
    }
  });
});
