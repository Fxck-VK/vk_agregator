// src/screens/NewJobPanel.tsx
import { useState } from "react";
import { Screen, Button } from "../ui/ui";
import { createJob } from "../api/client";

const OPS = [
  { id: "image_generate", label: "Изображение" },
  { id: "video_generate", label: "Видео" },
  { id: "text_generate", label: "Текст" },
];

export function NewJobPanel({ onCreated }: { onCreated: (id: string) => void }) {
  const [op, setOp] = useState(OPS[0].id);
  const [prompt, setPrompt] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  async function submit() {
    if (!prompt.trim() || busy) return;
    setBusy(true);
    setErr(null);
    try {
      const job = await createJob({ operation: op, prompt: prompt.trim() });
      setPrompt("");
      onCreated(job.id);
    } catch (e) {
      setErr(e instanceof Error ? e.message : "Не удалось создать задачу");
    } finally {
      setBusy(false);
    }
  }

  return (
    <Screen title="Создать" subtitle="Опишите, что нужно сгенерировать">
      <div className="segmented">
        {OPS.map((o) => (
          <button
            key={o.id}
            className={"segmented__item" + (op === o.id ? " is-active" : "")}
            onClick={() => setOp(o.id)}
          >
            {o.label}
          </button>
        ))}
      </div>
      <textarea
        className="input input--area"
        rows={5}
        placeholder="Например: минималистичный чёрно-белый постер с горами"
        value={prompt}
        onChange={(e) => setPrompt(e.target.value)}
      />
      {err && <p className="error">{err}</p>}
      <Button onClick={submit} disabled={busy || !prompt.trim()}>
        {busy ? "Создаём…" : "Сгенерировать"}
      </Button>
    </Screen>
  );
}
