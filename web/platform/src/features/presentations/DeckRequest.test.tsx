import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { DeckRequest } from "./DeckRequest";

const client = vi.hoisted(() => ({ read: vi.fn(), write: vi.fn() }));
vi.mock("@/lib/web-api/browser", () => ({ webBrowserFetch: client.read, webBrowserMutation: client.write }));
vi.mock("@/i18n/Link", () => ({
  default: ({ children, href }: { children: string; href: string }) => <a href={href}>{children}</a>,
}));

const conversation = "12345678-0000-4000-8000-012345678901";
const job = "12345678-0000-4000-8000-012345678902";
const prompt = "Сохранённый бриф";
const callbacks = () => ({ onRun: vi.fn(), onPrompt: vi.fn(), onResult: vi.fn(), onBusy: vi.fn(), onRestart: vi.fn() });

afterEach(() => { cleanup(); sessionStorage.clear(); vi.resetAllMocks(); vi.useRealTimers(); });

describe("presentation progress", () => {
  it("shows the actual provider stage for an existing run without creating a new job", async () => {
    const handlers = callbacks();
    client.read.mockImplementation(async () => Response.json({ job_id: job, status: "provider_processing", prompt, model_name: "Тестовая модель" }));
    render(<DeckRequest accountId="account" prompt="" run={{ conversation }} {...handlers} />);
    await waitFor(() => expect(screen.getByRole("status")).toHaveTextContent("Модель пишет слайды"));
    expect(screen.getByRole("listitem", { current: "step" })).toHaveTextContent("Модель пишет слайды");
    expect(screen.getByText("Тестовая модель")).toBeVisible();
    expect(handlers.onPrompt).toHaveBeenCalledWith(prompt);
    expect(client.write).not.toHaveBeenCalled();
  });

  it("reopens ready slides by URL without a new generation or quote", async () => {
    const handlers = callbacks();
    client.read.mockImplementation(async () => Response.json({ job_id: job, status: "succeeded", prompt, answer: "## Текст\n- Пункт" }));
    render(<DeckRequest accountId="account" prompt="" run={{ conversation, job }} {...handlers} />);
    await waitFor(() => expect(handlers.onResult).toHaveBeenCalledWith("## Текст\n- Пункт", prompt, job));
    expect(client.read).toHaveBeenCalledTimes(1);
    expect(client.write).not.toHaveBeenCalled();
  });

  it("gives a user-initiated status recheck a new time budget after timeout", async () => {
    vi.useFakeTimers();
    const handlers = callbacks();
    client.read.mockImplementation(async () => Response.json({ job_id: job, status: "provider_processing", prompt }));
    render(<DeckRequest accountId="account" prompt="" run={{ conversation, job }} {...handlers} />);
    await act(() => vi.advanceTimersByTimeAsync(180_001));
    const recheck = screen.getByRole("button", { name: "Проверить статус" });
    client.read.mockImplementation(async () => {
      await new Promise(resolve => setTimeout(resolve, 40));
      return Response.json({ job_id: job, status: "succeeded", prompt, answer: "Слайды" });
    });
    fireEvent.click(recheck);
    await act(() => vi.advanceTimersByTimeAsync(100));
    expect(handlers.onResult).toHaveBeenCalled();
    expect(client.write).not.toHaveBeenCalled();
  });

  it("reopens an accepted submission after the network response was lost", async () => {
    const handlers = callbacks();
    client.read.mockImplementation(async () => Response.json({ model_id: "fixture", model_name: "Модель", credits: 5 }));
    client.write.mockResolvedValueOnce(Response.json({ id: conversation, title: "", created_at: "2026-01-01T00:00:00Z", updated_at: "2026-01-01T00:00:00Z" }))
      .mockRejectedValueOnce(new Error("connection closed"));
    render(<DeckRequest accountId="account" prompt={prompt} {...handlers} />);
    fireEvent.click(await screen.findByRole("button", { name: "Сгенерировать слайды" }));
    await waitFor(() => expect(handlers.onRun).toHaveBeenCalledWith({ conversation, pending: true }));
    expect(new URL(window.location.href).searchParams.get("run")).toBe(conversation);
    expect(client.write).toHaveBeenCalledTimes(2);
    window.history.replaceState(null, "", "/");
  });

  it("shows a recoverable message if a legacy completed job has no stored answer", async () => {
    const handlers = callbacks();
    client.read.mockImplementation(async () => Response.json({ job_id: job, status: "succeeded", prompt, failure_code: "answer_missing" }));
    render(<DeckRequest accountId="account" prompt="" run={{ conversation, job }} {...handlers} />);
    expect(await screen.findByRole("button", { name: "Проверить статус" })).toBeVisible();
    expect(screen.getByRole("status")).toHaveTextContent("Текст слайдов пока недоступен");
    expect(handlers.onResult).not.toHaveBeenCalled();
    expect(client.write).not.toHaveBeenCalled();
  });
});
