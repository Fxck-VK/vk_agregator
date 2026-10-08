import { StrictMode } from "react";
import { fireEvent, render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { webBrowserFetch } from "@/lib/web-api/browser";
import { InviteEntry } from "./InviteEntry";

const router = { replace: vi.fn() };
vi.mock("@/i18n/navigation", () => ({ useRouter: () => router }));
vi.mock("@/lib/web-api/browser", () => ({ webBrowserFetch: vi.fn() }));
afterEach(() => vi.clearAllMocks());

it("saves the invitation once across strict-mode effects before continuing", async () => {
 vi.mocked(webBrowserFetch).mockResolvedValue(new Response(null, { status: 204 }));
 render(<StrictMode><InviteEntry code="INVITE1234" /></StrictMode>);
 fireEvent.click(await screen.findByRole("button", { name: "Перейти к регистрации" }));
 expect(webBrowserFetch).toHaveBeenCalledTimes(1);
 expect(webBrowserFetch).toHaveBeenCalledWith("/web/v1/referrals/visit", expect.objectContaining({ method: "POST", body: JSON.stringify({ code: "INVITE1234" }) }));
 expect(router.replace).toHaveBeenCalledWith("/login");
});
it("shows capture failure instead of claiming the invitation was saved", async () => {
 vi.mocked(webBrowserFetch).mockResolvedValue(new Response(null, { status: 400 }));
 render(<InviteEntry code="UNKNOWN123" />);
 expect(await screen.findByRole("alert")).toHaveTextContent("Не удалось сохранить приглашение");
 expect(screen.queryByText(/Приглашение сохранено/)).not.toBeInTheDocument();
 expect(screen.getByRole("button", { name: "Повторить" })).toBeInTheDocument();
});
