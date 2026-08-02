import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("next/navigation", () => ({
  usePathname: vi.fn(),
  useRouter: vi.fn(),
}));

import { usePathname, useRouter } from "next/navigation";

import { ru } from "@/i18n/ru";
import type { ConversationItem } from "@/lib/web-api/contracts";

import { SidebarConversations } from "./SidebarConversations";

const conversations: ConversationItem[] = [
  {
    id: "d7c979f5-24e5-4f88-924b-a592d6e5a906",
    title: "Подготовить макет",
    created_at: "2026-07-31T09:00:00Z",
    updated_at: "2026-07-31T09:05:00Z",
  },
  {
    id: "a2a006fc-4457-4bb5-bc4d-4f553d51766b",
    title: "   ",
    created_at: "2026-07-31T09:10:00Z",
    updated_at: "2026-07-31T09:15:00Z",
  },
];

describe("SidebarConversations", () => {
  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it("marks only the exact current conversation and always renders the create action", () => {
    vi.mocked(usePathname).mockReturnValue("/app/chat/d7c979f5-24e5-4f88-924b-a592d6e5a906");
    vi.mocked(useRouter).mockReturnValue({ push: vi.fn(), refresh: vi.fn() } as never);

    render(<SidebarConversations conversations={conversations} />);

    expect(screen.getByRole("link", { name: conversations[0].title })).toHaveAttribute("aria-current", "page");
    expect(screen.getByRole("link", { name: ru.conversations.unnamed })).not.toHaveAttribute("aria-current");
    expect(screen.getByRole("button", { name: ru.conversations.createLabel })).toBeEnabled();
  });

  it("does not mark a nested conversation route as active", () => {
    vi.mocked(usePathname).mockReturnValue("/app/chat/d7c979f5-24e5-4f88-924b-a592d6e5a906/child");
    vi.mocked(useRouter).mockReturnValue({ push: vi.fn(), refresh: vi.fn() } as never);

    render(<SidebarConversations conversations={conversations} />);

    expect(screen.getByRole("link", { name: conversations[0].title })).not.toHaveAttribute("aria-current");
  });

  it("renders safe conversation titles as local chat links and uses the unnamed fallback", () => {
    render(<SidebarConversations conversations={conversations} />);

    expect(screen.getByRole("heading", { name: ru.conversations.recentHeading })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Подготовить макет" })).toHaveAttribute(
      "href",
      "/app/chat/d7c979f5-24e5-4f88-924b-a592d6e5a906",
    );
    expect(screen.getByRole("link", { name: ru.conversations.unnamed })).toHaveAttribute(
      "href",
      "/app/chat/a2a006fc-4457-4bb5-bc4d-4f553d51766b",
    );
    expect(screen.queryByText(conversations[0].created_at)).not.toBeInTheDocument();
    expect(screen.queryByText(conversations[0].updated_at)).not.toBeInTheDocument();
  });

  it("renders each recent chat through an independently labelled action control", () => {
    vi.mocked(usePathname).mockReturnValue("/app");
    vi.mocked(useRouter).mockReturnValue({ refresh: vi.fn(), replace: vi.fn() } as never);

    render(<SidebarConversations conversations={conversations} />);

    expect(screen.getAllByRole("button", { name: ru.conversations.actionsLabel })).toHaveLength(conversations.length);
  });

  it("renders an explicit empty recent-chat state", () => {
    vi.mocked(usePathname).mockReturnValue("/app/chat");
    vi.mocked(useRouter).mockReturnValue({ push: vi.fn(), refresh: vi.fn() } as never);

    render(<SidebarConversations conversations={[]} />);

    expect(screen.getByText(ru.conversations.empty)).toBeInTheDocument();
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: ru.conversations.createLabel })).toBeEnabled();
  });
});
