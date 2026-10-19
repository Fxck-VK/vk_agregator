import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { SpeechWorkspace } from "./SpeechWorkspace";
import { loadModelCatalog } from "@/features/models/model-catalog-cache";
import { activateSpeech, loadSpeechJob, loadSpeechResult, prepareSpeech } from "./speech-api";
import type { PublicCatalog } from "@/features/models/model-catalog-contract";
import { LocaleProvider } from "@/i18n/LocaleProvider";
import { uploadMusicInput } from "@/features/music/music-api";

vi.mock("@/features/models/model-catalog-cache", () => ({ loadModelCatalog: vi.fn(), resetModelCatalogCacheForTests: vi.fn() }));
vi.mock("./speech-api", () => ({ prepareSpeech: vi.fn(), activateSpeech: vi.fn(), loadSpeechJob: vi.fn(), loadSpeechResult: vi.fn() }));
vi.mock("@/features/music/music-api",()=>({uploadMusicInput:vi.fn()}));

describe("Speech workspace admission", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(loadModelCatalog).mockResolvedValue({ schema_version: 1, default_model_id: "gpt_4o_mini_tts", items: [
      { id: "gpt_4o_mini_tts", name: "GPT-4o Mini TTS", kind: "audio", description: "", categories: [], verification: "pending-verification", operations: [{ id: "speak", kind: "audio", enabled: false, inputs: { images: { support: "unsupported", enabled: false }, video: { support: "unsupported", enabled: false }, audio: { support: "unsupported", enabled: false }, documents: { support: "unsupported", enabled: false }, max_total_bytes: 0 }, audio: { tasks: ["tts"] } }] },
    ] } satisfies PublicCatalog);
  });
  it("opens enabled Whisper and confirms only the server price after an owned upload", async () => {
    const catalog=await loadModelCatalog();
    const base=catalog.items[0];
    vi.mocked(loadModelCatalog).mockResolvedValue({...catalog,items:[...catalog.items,{...base,id:"whisper_1",name:"Whisper 1",verification:"dev-smoke",operations:[{...base.operations[0],id:"transcribe",enabled:true,audio:{tasks:["transcribe"],formats:["json"],credits_per_started_minute:25},inputs:{...base.operations[0].inputs,audio:{support:"supported",enabled:true,max_bytes:25_000_000,max_duration_sec:480}}}]}]});
    vi.mocked(uploadMusicInput).mockResolvedValue({artifact_id:"11111111-1111-4111-8111-111111111111",mime_type:"audio/wav",duration_ms:60001,size_bytes:100});
    const job={id:"22222222-2222-4222-8222-222222222222",model_id:"whisper_1",status:"prepared",cost_estimate:50,created_at:"2026-10-19T00:00:00Z"};
    vi.mocked(prepareSpeech).mockResolvedValue({job,balance:100,can_afford:true});
    render(<SpeechWorkspace />);
    const file=await screen.findByLabelText("Аудиозапись");
    expect(file).toBeEnabled();
    expect(screen.getByText("25 кредитов за каждую начатую минуту.")).toBeVisible();
    fireEvent.change(file,{target:{files:[new File(["synthetic"],"speech.wav",{type:"audio/wav"})]}});
    fireEvent.click(screen.getByRole("button",{name:"Рассчитать стоимость"}));
    await screen.findByRole("button",{name:"Подтвердить запуск"});
    expect(prepareSpeech).toHaveBeenCalledWith({model_id:"whisper_1",audio_artifact_id:"11111111-1111-4111-8111-111111111111",speech:{format:"json"}},expect.any(String));
    expect(activateSpeech).not.toHaveBeenCalled();
    expect(screen.getByRole("combobox",{name:"Модель"})).toHaveValue("whisper_1");
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

  it("waits offline after activation and resumes status/result reads without repeating the write", async () => {
    const catalog = await loadModelCatalog();
    vi.mocked(loadModelCatalog).mockResolvedValue({ ...catalog, items: catalog.items.map((model) => ({
      ...model, verification: "verified-contract", operations: model.operations.map((operation) => ({ ...operation, enabled: true })),
    })) });
    const job = { id: "22222222-2222-4222-8222-222222222222", model_id: "gpt_4o_mini_tts", status: "queued", cost_estimate: 25, created_at: "2026-09-20T00:00:00Z" };
    vi.mocked(prepareSpeech).mockResolvedValue({ job, balance: 100, can_afford: true });
    vi.mocked(activateSpeech).mockResolvedValue(job);
    vi.mocked(loadSpeechJob).mockResolvedValue({ ...job, status: "succeeded" });
    vi.mocked(loadSpeechResult).mockResolvedValue([{ id: job.id, kind: "audio", format: "wav", url: `/web/v1/speech-artifacts/${job.id}` }]);
    const { unmount } = render(<SpeechWorkspace />);
    fireEvent.change(screen.getByRole("textbox", { name: "Текст" }), { target: { value: "synthetic" } });
    const calculate = screen.getByRole("button", { name: "Рассчитать стоимость" });
    await waitFor(() => expect(calculate).toBeEnabled());
    fireEvent.click(calculate);
    const confirm = await screen.findByRole("button", { name: "Подтвердить запуск" });
    vi.useFakeTimers();
    try {
      await act(async () => {
        Object.defineProperty(navigator, "onLine", { configurable: true, value: false });
        window.dispatchEvent(new Event("offline"));
        fireEvent.click(confirm);
      });
      await act(async () => { await vi.advanceTimersByTimeAsync(60000); });
      expect(screen.getByText("Задание выполняется…")).toBeVisible();
      expect(loadSpeechJob).not.toHaveBeenCalled();
      expect(screen.queryByRole("alert")).toBeNull();
      await act(async () => {
        Object.defineProperty(navigator, "onLine", { configurable: true, value: true });
        window.dispatchEvent(new Event("online"));
        window.dispatchEvent(new Event("online"));
      });
      expect(loadSpeechJob).toHaveBeenCalledTimes(1);
      expect(loadSpeechJob).toHaveBeenCalledWith(job.id, expect.any(AbortSignal));
      expect(loadSpeechResult).toHaveBeenCalledWith(job.id, expect.any(AbortSignal));
      expect(screen.getByRole("link", { name: "Скачать аудио" })).toBeVisible();
      expect(prepareSpeech).toHaveBeenCalledTimes(1);
      expect(activateSpeech).toHaveBeenCalledTimes(1);
    } finally {
      unmount();
      Object.defineProperty(navigator, "onLine", { configurable: true, value: true });
      vi.useRealTimers();
    }
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
