vi.mock("server-only", () => ({}));
vi.mock("next/headers", () => ({ headers: vi.fn(async () => new Headers()), cookies: vi.fn(async () => ({ get: () => undefined })) }));
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
  generateMetadata as generatePrivateMetadata,
  revalidate as privateRevalidate,
} from "./app/layout";

describe("route surface boundaries", () => {
  it("keeps the public surface indexable", () => {
    expect(publicMetadata.robots).toEqual({ index: true, follow: true });
  });

  it("keeps the public surface inside the public shell", async () => {
    const layout = await PublicLayout({ children: "Public content" });

    expect(layout.type.name).toBe("LocalizedPublicShell");
    expect(layout.props.children).toBe("Public content");
  });

  it("keeps the authenticated app dynamic and non-indexable", async () => {
    expect(privateDynamic).toBe("force-dynamic");
    expect(privateRevalidate).toBe(0);
    expect((await generatePrivateMetadata()).robots).toEqual({ index: false, follow: false });
  });
});
