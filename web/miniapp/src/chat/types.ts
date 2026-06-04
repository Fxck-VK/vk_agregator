// src/chat/types.ts
export type Role = "user" | "bot";

export interface ChatMessage {
  id: string;
  role: Role;
  text?: string;
  operation?: string;
  jobId?: string;
  status?: string;
  pending?: boolean;
  artifactIds?: string[];
  error?: string;
}

export interface ChatModel {
  id: string;
  label: string;
  operation: string;
}

export const MODELS: ChatModel[] = [
  { id: "text", label: "Текст", operation: "text_generate" },
  { id: "image", label: "Изображение", operation: "image_generate" },
  { id: "video", label: "Видео", operation: "video_generate" },
];

export function uid(): string {
  return Math.random().toString(36).slice(2) + Date.now().toString(36);
}
