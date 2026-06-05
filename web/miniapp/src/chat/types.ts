// src/chat/types.ts
export type Role = "user" | "bot";

export interface ChatMessage {
  id: string;
  role: Role;
  text?: string;
  operation?: string;
  model?: string;
  jobId?: string;
  status?: string;
  pending?: boolean;
  artifactIds?: string[];
  error?: string;
  createdAt?: string;
}

export type ModalityId = "text" | "image" | "video";

export interface AiModel {
  id: string;
  label: string;
}

export interface ModalityDef {
  id: ModalityId;
  label: string;
  operation: string; // text_generate | image_generate | video_generate
  models: AiModel[];
}

export const MODALITIES: ModalityDef[] = [
  {
    id: "text",
    label: "Текст",
    operation: "text_generate",
    models: [
      { id: "gpt-4o-mini", label: "GPT-4o mini" },
      { id: "gpt-4o", label: "GPT-4o" },
      { id: "llama-3.1", label: "Llama 3.1" },
    ],
  },
  {
    id: "image",
    label: "Фото",
    operation: "image_generate",
    models: [
      { id: "sdxl", label: "Stable Diffusion XL" },
      { id: "kandinsky", label: "Kandinsky 3" },
    ],
  },
  {
    id: "video",
    label: "Видео",
    operation: "video_generate",
    models: [{ id: "kling", label: "Kling" }],
  },
];

export function modalityById(id: ModalityId): ModalityDef {
  return MODALITIES.find((m) => m.id === id) ?? MODALITIES[0];
}

export function modalityByOperation(operation: string): ModalityDef {
  return MODALITIES.find((m) => m.operation === operation) ?? MODALITIES[0];
}

export interface Chat {
  id: string;
  title: string;
  createdAt: number;
  updatedAt: number;
  messages: ChatMessage[];
}

export function uid(): string {
  return Math.random().toString(36).slice(2) + Date.now().toString(36);
}
