"use client";

import { useEffect, useState } from "react";
import { z } from "zod";
import styles from "./TextModelSelector.module.css";
import { webBrowserFetch } from "@/lib/web-api/browser";

const modelSchema = z.object({
  id: z.enum(["chatgpt", "gpt_5_5", "claude_opus_4_7", "gemini_3_1_pro", "claude_opus_4_8", "gpt_5_6_terra", "gpt_6_astra", "claude_opus_5", "gemini_3_7_flash", "claude_fable_5_1", "claude_fable_5", "gemini_3_6_flash"]),
  name: z.string().min(1).max(80),
  estimate_credits: z.number().int().nonnegative(),
  max_prompt_bytes: z.number().int().positive().optional(),
  max_output_tokens: z.number().int().positive().optional(),
}).strict();
export type TextModel = z.infer<typeof modelSchema>;

export function TextModelSelector({ disabled, onChange }: { disabled: boolean; onChange: (model: TextModel) => void }) {
  const [models, setModels] = useState<TextModel[]>([{ id: "chatgpt", name: "НейроХаб", estimate_credits: 0 }]);
  const [selected, setSelected] = useState("chatgpt");
  const [failed, setFailed] = useState(false);
  useEffect(() => {
    let active = true;
    void (async () => {
      try {
        const response = await webBrowserFetch("/web/v1/text-models");
        if (!response.ok) throw new Error("catalog unavailable");
        const parsed = z.object({ items: z.array(modelSchema).min(1).max(12) }).strict().parse(await response.json());
        if (active) setModels(parsed.items);
      } catch { if (active) setFailed(true); }
    })();
    return () => { active = false; };
  }, []);
  const current = models.find((model) => model.id === selected);
  return <div className={styles.selector}>
    <label>Модель{" "}<select aria-label="Текстовая модель" disabled={disabled} value={selected} onChange={(event) => {
      const model = models.find((item) => item.id === event.target.value);
      if (model) { setSelected(model.id); onChange(model); }
    }}>
      {models.map((model) => <option key={model.id} value={model.id}>{model.name}{model.estimate_credits > 0 ? ` · ${model.estimate_credits} кредитов за ответ` : ""}</option>)}
    </select></label>
    {current && current.estimate_credits > 0 && <p>{current.estimate_credits} кредитов за ответ · до {current.max_output_tokens} токенов ответа. При длинном диалоге может потребоваться новый чат.</p>}
    {failed && <small>Каталог дополнительных моделей временно недоступен.</small>}
  </div>;
}
