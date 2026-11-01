import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
vi.mock("@/features/auth/login-page.server", () => ({ loadLoginPageProps: vi.fn() }));
vi.mock("@/features/auth/LoginWindow/LoginWindow", () => ({ LoginWindow: ({ children }: { children: React.ReactNode }) => <>{children}</> }));
vi.mock("@/features/auth/LoginForm/LoginForm", () => ({ LoginForm: ({ previewAccount }: { previewAccount?: string }) => <p>Real form {previewAccount ?? "default"}</p> }));
import { loadLoginPageProps } from "@/features/auth/login-page.server";
import { AuthEntry } from "./AuthEntry";
afterEach(() => { cleanup(); vi.clearAllMocks(); });

it.each([["provider", "yandex"], ["vk", "vk"], ["email", "default"]])("allows the %s demo only when server preview is enabled", async (scenario, expected) => {
  vi.mocked(loadLoginPageProps).mockResolvedValue({ preview: true, entry: "choice", methods: { registration: false, password: true, recovery: false, email_link: false, phone_link: false, providers: [] }, initialMode: "login", legalMetadata: {} as never, oauthFailed: false });
  render(await AuthEntry({ searchParams: Promise.resolve({ demo: scenario }) }));
  expect(screen.getByText(`Real form ${expected}`)).toBeInTheDocument();
  expect(screen.queryByText(/Preview/)).not.toBeInTheDocument();
});
it("ignores demo query on non-preview server", async () => {
  vi.mocked(loadLoginPageProps).mockResolvedValue({ preview: false, entry: "choice", methods: { registration: false, password: true, recovery: false, email_link: false, phone_link: false, providers: [] }, initialMode: "login", legalMetadata: {} as never, oauthFailed: false });
  render(await AuthEntry({ searchParams: Promise.resolve({ demo: "provider" }) }));
  expect(screen.getByText("Real form default")).toBeInTheDocument();
  expect(screen.queryByText(/Preview/)).not.toBeInTheDocument();
});
