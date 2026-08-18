import { describe, expect, it, vi } from "vitest";

vi.mock("next/navigation", () => ({
  redirect: vi.fn(),
  usePathname: vi.fn(),
  useRouter: vi.fn(),
  useSearchParams: vi.fn(() => new URLSearchParams()),
}));

vi.mock("@/features/session/session-data", () => ({
  loadWorkspaceSession: vi.fn(),
}));

import { metadata as publicMetadata } from "./(public)/layout";
import PublicLayout from "./(public)/layout";
import {
  dynamic as privateDynamic,
  metadata as privateMetadata,
  revalidate as privateRevalidate,
} from "./app/layout";

describe("route surface boundaries", () => {
  it("keeps the public surface indexable", () => {
    expect(publicMetadata.robots).toEqual({ index: true, follow: true });
  });

  it("keeps the public surface inside the public shell", () => {
    const layout = PublicLayout({ children: "Public content" });

    expect(layout.type.name).toBe("PublicShell");
    expect(layout.props.children).toBe("Public content");
  });

  it("keeps the authenticated app dynamic and non-indexable", () => {
    expect(privateDynamic).toBe("force-dynamic");
    expect(privateRevalidate).toBe(0);
    expect(privateMetadata.robots).toEqual({ index: false, follow: false });
  });
});
