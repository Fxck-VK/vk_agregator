"use client";

import { type FormEvent, type KeyboardEvent, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";

import { useLandingToolSelection } from "../LandingToolSelection/LandingToolSelection";
import { saveGuestDraft } from "./guest-draft";
import styles from "./HeroComposer.module.css";

const promptLimit = 4_000;

export function HeroComposer() {
  const router = useRouter();
  const { selectedTool } = useLandingToolSelection();
  const [prompt, setPrompt] = useState("");
  const [quality, setQuality] = useState("1K");
  const isImageTool = selectedTool.kind === "image";
  const price = selectedTool.priceStarsByQuality?.[quality];
  const destination = isImageTool ? "/login?next=/app/image" : "/login?next=/app/chats";

  const submit = () => {
    if (!prompt.trim()) return;
    saveGuestDraft(prompt, isImageTool ? "image" : "chat");
    router.push(destination);
  };

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    submit();
  };

  const onPromptKeyDown = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    if (event.key === "Enter" && !event.shiftKey) {
      event.preventDefault();
      submit();
    }
  };

  return (
    <form className={styles.composer} onSubmit={onSubmit}>
      <div className={styles.promptField}>
        <label htmlFor="landing-prompt">
          {isImageTool ? "Описание изображения" : "Задайте вопрос NeiroHub"}
        </label>
        <textarea
          id="landing-prompt"
          maxLength={promptLimit}
          onChange={(event) => setPrompt(event.target.value)}
          onKeyDown={onPromptKeyDown}
          placeholder={isImageTool ? "Например: кинематографичный город будущего на закате" : "Напишите, что хотите сделать"}
          rows={4}
          value={prompt}
        />
      </div>

      {isImageTool ? (
        <div className={styles.imageOptions}>
          <label className={styles.referenceInput}>
            <span>Референс</span>
            <input accept="image/*" aria-label="Добавить референс" type="file" />
          </label>
          <label>
            <span>Качество</span>
            <select aria-label="Качество" onChange={(event) => setQuality(event.target.value)} value={quality}>
              <option value="1K">1K</option>
              <option value="2K">2K</option>
            </select>
          </label>
        </div>
      ) : (
        <Link className={styles.attachmentLink} href="/login?next=/app/chats" prefetch={false}>
          <span aria-hidden="true">＋</span> Добавить файл после входа
        </Link>
      )}

      <div className={styles.actions}>
        <p>{isImageTool && price !== undefined ? `Стоимость: ${price} ★` : "Enter — отправить, Shift+Enter — новая строка"}</p>
        <button disabled={!prompt.trim()} type="submit">
          {isImageTool
            ? `Создать изображение${price !== undefined ? ` · ${price} ★` : ""}`
            : "Начать чат"}
        </button>
      </div>
    </form>
  );
}
