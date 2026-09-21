import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { SpeechWorkspace } from "./SpeechWorkspace";
import { loadModelCatalog } from "@/features/models/model-catalog-cache";
import { activateSpeech, loadSpeechJob, prepareSpeech } from "./speech-api";
import type { PublicCatalog } from "@/features/models/model-catalog-contract";
import { LocaleProvider } from "@/i18n/LocaleProvider";

vi.mock("@/features/models/model-catalog-cache", () => ({ loadModelCatalog: vi.fn(), resetModelCatalogCacheForTests: vi.fn() }));
vi.mock("./speech-api", () => ({ prepareSpeech: vi.fn(), activateSpeech: vi.fn(), loadSpeechJob: vi.fn(), loadSpeechResult: vi.fn() }));

describe("Speech workspace admission", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(loadModelCatalog).mockResolvedValue({ schema_version: 1, default_model_id: "gpt_4o_mini_tts", items: [
      { id: "gpt_4o_mini_tts", name: "GPT-4o Mini TTS", kind: "audio", description: "", categories: [], verification: "pending-verification", operations: [{ id: "speak", kind: "audio", enabled: false, inputs: { images: { support: "unsupported", enabled: false }, video: { support: "unsupported", enabled: false }, audio: { support: "unsupported", enabled: false }, documents: { support: "unsupported", enabled: false }, max_total_bytes: 0 }, audio: { tasks: ["tts"] } }] },
    ] } satisfies PublicCatalog);
  });
  it("shows pending speech separately and cannot submit a paid request", async () => {
    render(<SpeechWorkspace />);
    await screen.findByText(/Списание отключено/);
    fireEvent.change(screen.getByRole("textbox", { name: "Текст" }), { target: { value: "synthetic" } });
    expect(screen.getByRole("button", { name: "Рассчитать стоимость" })).toBeDisabled();
    expect(screen.queryByText("Max mode")).not.toBeInTheDocument();
    expect(prepareSpeech).not.toHaveBeenCalled();
    fireEvent.change(screen.getByRole("combobox", { name: "Модель" }), { target: { value: "whisper_1" } });
    await waitFor(() => expect(screen.getByLabelText("Аудиозапись")).toBeDisabled());
    expect(screen.getByText(/MP3 или WAV/)).toBeInTheDocument();
  });

  it("switches labels and safe errors without resetting the speech draft or refetching", async () => {
    vi.mocked(loadModelCatalog).mockRejectedValue(new Error("private upstream detail"));
    const { rerender } = render(<LocaleProvider locale="ru"><SpeechWorkspace /></LocaleProvider>);
    fireEvent.change(screen.getByRole("textbox", { name: "Текст" }), { target: { value: "Мой текст" } });
    await screen.findByText("Не удалось загрузить модели.");
    rerender(<LocaleProvider locale="en"><SpeechWorkspace /></LocaleProvider>);
    expect(screen.getByRole("alert")).toHaveTextContent("Could not load models.");
    expect(screen.getByRole("heading", { name: "Speech generation and transcription" })).toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: "Text" })).toHaveValue("Мой текст");
    expect(screen.getByRole("button", { name: "Calculate cost" })).toBeDisabled();
    expect(loadModelCatalog).toHaveBeenCalledTimes(1);
    expect(screen.queryByText("private upstream detail")).not.toBeInTheDocument();
  });

  it("retries an uncertain activation with the same prepared job and no second preparation", async () => {
    // Explicit local fixture: does not grant actual provider admission or pricing.
    const catalog = await loadModelCatalog();
    vi.mocked(loadModelCatalog).mockResolvedValue({ ...catalog, items: catalog.items.map((model) => ({
      ...model, verification: "verified-contract", operations: model.operations.map((operation) => ({ ...operation, enabled: true })),
    })) });
    const job = { id: "22222222-2222-4222-8222-222222222222", model_id: "gpt_4o_mini_tts", status: "prepared", cost_estimate: 25, created_at: "2026-09-20T00:00:00Z" };
    vi.mocked(prepareSpeech).mockResolvedValue({ job, balance: 100, can_afford: true });
    vi.mocked(activateSpeech).mockRejectedValueOnce(new Error("Ответ потерян. Повторите подтверждение этого задания."))
      .mockResolvedValueOnce({ ...job, status: "queued" });
    vi.mocked(loadSpeechJob).mockResolvedValue({ ...job, status: "failed_terminal" });
    render(<SpeechWorkspace />);
    fireEvent.change(screen.getByRole("textbox", { name: "Текст" }), { target: { value: "synthetic" } });
    const calculate = screen.getByRole("button", { name: "Рассчитать стоимость" });
    await waitFor(() => expect(calculate).toBeEnabled());
    fireEvent.click(calculate);
    fireEvent.click(await screen.findByRole("button", { name: "Подтвердить запуск" }));
    await screen.findByRole("alert");
    const confirm = screen.getByRole("button", { name: "Подтвердить запуск" });
    await waitFor(() => expect(confirm).toBeEnabled());
    fireEvent.click(confirm);
    await screen.findByText("Задание завершилось без результата.");
    expect(prepareSpeech).toHaveBeenCalledTimes(1);
    expect(activateSpeech).toHaveBeenCalledTimes(2);
    expect(activateSpeech).toHaveBeenNthCalledWith(1, job.id);
    expect(activateSpeech).toHaveBeenNthCalledWith(2, job.id);
  });
});
