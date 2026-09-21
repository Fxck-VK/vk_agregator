"use client";

import { useEffect, useRef, useState } from "react";
import { WorkspacePageFrame } from "@/components/layout/WorkspacePageFrame/WorkspacePageFrame";
import { Button } from "@/components/ui/Button/Button";
import { InputSurface } from "@/components/ui/InputSurface/InputSurface";
import { CreditAmount } from "@/components/ui/CreditAmount/CreditAmount";
import { loadModelCatalog } from "@/features/models/model-catalog-cache";
import type { PublicCatalogModel } from "@/features/models/model-catalog-contract";
import { uploadMusicInput } from "@/features/music/music-api";
import { activateSpeech, loadSpeechJob, loadSpeechResult, prepareSpeech, type SpeechArtifact, type SpeechPreparation, type SpeechRequest } from "./speech-api";
import styles from "./SpeechWorkspace.module.css";

export function SpeechWorkspace() {
  const [models, setModels] = useState<PublicCatalogModel[] | null>(null);
  const [modelId, setModelId] = useState<SpeechRequest["model_id"]>("gpt_4o_mini_tts");
  const [text, setText] = useState("");
  const [voice, setVoice] = useState("alloy");
  const [speed, setSpeed] = useState(1);
  const [language, setLanguage] = useState("");
  const [file, setFile] = useState<File | null>(null);
  const [prepared, setPrepared] = useState<SpeechPreparation | null>(null);
  const [jobId, setJobId] = useState<string | null>(null);
  const [results, setResults] = useState<SpeechArtifact[]>([]);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const inFlight = useRef(false);
  const pending = useRef<{ body: SpeechRequest; key: string } | null>(null);
  const tts = modelId === "gpt_4o_mini_tts";
  const model = models?.find((item) => item.id === modelId);
  const operation = model?.operations.find((item) => item.id === (tts ? "speak" : "transcribe"));
  const enabled = model?.verification !== "pending-verification" && operation?.enabled === true;
  const locked = busy || jobId !== null;

  useEffect(() => {
    let cancelled = false;
    loadModelCatalog().then((catalog) => { if (!cancelled) setModels(catalog.items.filter((m) => m.id === "gpt_4o_mini_tts" || m.id === "whisper_1")); })
      .catch(() => { if (!cancelled) setError("Не удалось загрузить модели."); });
    return () => { cancelled = true; };
  }, []);

  useEffect(() => {
    if (jobId === null) return;
    let cancelled = false;
    let timer: ReturnType<typeof setTimeout>;
    const poll = async () => {
      try {
        const job = await loadSpeechJob(jobId);
        if (cancelled) return;
        if (job.status === "succeeded") {
          const artifacts = await loadSpeechResult(jobId);
          if (!cancelled) { setResults(artifacts); setJobId(null); }
          return;
        }
        if (["failed_terminal", "cancelled", "expired", "refunded", "rejected"].includes(job.status)) {
          setError("Задание завершилось без результата."); setJobId(null); return;
        }
      } catch { if (!cancelled) setError("Проверка статуса временно недоступна. Продолжаем проверять это задание."); }
      if (!cancelled) timer = setTimeout(poll, 2500);
    };
    void poll();
    return () => { cancelled = true; clearTimeout(timer); };
  }, [jobId]);

  const change = (action: () => void) => { setPrepared(null); pending.current = null; setError(""); action(); };
  const prepare = async () => {
    if (!enabled || inFlight.current || jobId !== null) return;
    inFlight.current = true; setBusy(true); setError("");
    try {
      if (pending.current === null) {
        let body: SpeechRequest;
        if (tts) body = { model_id: modelId, speech: { text, voice, speed, format: "wav" } };
        else {
          if (!file) throw new Error("Выберите MP3 или WAV.");
          const upload = await uploadMusicInput(file);
          body = { model_id: modelId, audio_artifact_id: upload.artifact_id, speech: { format: "json", ...(language ? { language } : {}) } };
        }
        pending.current = { body, key: crypto.randomUUID() };
      }
      setPrepared(await prepareSpeech(pending.current.body, pending.current.key));
    } catch (cause) { setError(cause instanceof Error ? cause.message : "Не удалось подготовить задание."); }
    finally { inFlight.current = false; setBusy(false); }
  };
  const activate = async () => {
    if (!prepared || !prepared.can_afford || !enabled || inFlight.current) return;
    inFlight.current = true; setBusy(true); setError("");
    try { const job = await activateSpeech(prepared.job.id); setJobId(job.id); setPrepared(null); pending.current = null; }
    catch (cause) { setError(cause instanceof Error ? cause.message : "Не удалось подтвердить запуск."); }
    finally { inFlight.current = false; setBusy(false); }
  };

  return <WorkspacePageFrame><section className={styles.workspace} aria-label="Речь">
    <h1>Озвучка и распознавание</h1>
    <p>Создайте аудио из текста или получите текст из записи.</p>
    <label className={styles.field}>Модель<InputSurface><select disabled={locked} value={modelId} onChange={(e) => change(() => setModelId(e.target.value as SpeechRequest["model_id"]))}>
      <option value="gpt_4o_mini_tts">GPT-4o Mini TTS — озвучка</option><option value="whisper_1">Whisper-1 — распознавание</option>
    </select></InputSurface></label>
    {!enabled ? <p role="status">{models === null ? "Загружаем каталог…" : "Запуск пока недоступен: проверяем модель и стоимость. Списание отключено."}</p> : null}
    {tts ? <>
      <label className={styles.field}>Текст<InputSurface><textarea rows={6} maxLength={4096} disabled={locked} value={text} onChange={(e) => change(() => setText(e.target.value))} /></InputSurface></label>
      <div className={styles.row}>
        <label className={styles.field}>Голос<InputSurface><select value={voice} disabled={locked} onChange={(e) => change(() => setVoice(e.target.value))}>{["alloy", "echo", "fable", "onyx", "nova", "shimmer"].map((v) => <option key={v}>{v}</option>)}</select></InputSurface></label>
        <label className={styles.field}>Скорость<InputSurface><input type="number" min={0.25} max={4} step={0.25} value={speed} disabled={locked} onChange={(e) => change(() => setSpeed(Number(e.target.value)))} /></InputSurface></label>
      </div><p>Результат: WAV. До 4096 символов.</p>
    </> : <>
      <label className={styles.field}>Аудиозапись<input type="file" accept=".mp3,.wav,audio/mpeg,audio/wav" disabled={locked || !enabled} onChange={(e) => change(() => setFile(e.target.files?.[0] ?? null))} /></label>
      <p>MP3 или WAV, до 25 МБ и 8 минут.</p>
      <label className={styles.field}>Язык<InputSurface><select disabled={locked} value={language} onChange={(e) => change(() => setLanguage(e.target.value))}><option value="">Определить автоматически</option><option value="ru">Русский</option><option value="en">Английский</option></select></InputSurface></label>
    </>}
    {error ? <p role="alert">{error}</p> : null}
    {jobId ? <p role="status">Задание выполняется…</p> : null}
    {prepared ? <div className={styles.row}><CreditAmount prefix="Стоимость:" value={prepared.job.cost_estimate} /><Button disabled={locked || !prepared.can_afford} onClick={() => void activate()}>Подтвердить запуск</Button>{!prepared.can_afford ? <span>Недостаточно кредитов.</span> : null}</div>
      : <Button disabled={locked || !enabled || (tts ? !text.trim() || speed < 0.25 || speed > 4 : !file || file.size > 25 * 1024 * 1024)} onClick={() => void prepare()}>Рассчитать стоимость</Button>}
    {results.map((artifact) => <div key={artifact.id}>{artifact.kind === "audio" ? <audio controls src={artifact.url} /> : null}<a href={artifact.url} download>{artifact.kind === "audio" ? "Скачать аудио" : "Скачать текст"}</a></div>)}
  </section></WorkspacePageFrame>;
}
