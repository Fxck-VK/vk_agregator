"use client";

import { useState, type FormEvent } from "react";
import { useRouter } from "next/navigation";

import { Button } from "@/components/ui/Button/Button";
import { ru } from "@/i18n/ru";
import { webBrowserMutation } from "@/lib/web-api/browser";
import { parseConversationList, parseWebChatJob } from "@/lib/web-api/contracts";

import styles from "./WorkspacePrompt.module.css";

export function WorkspacePrompt() {
  const router = useRouter();
  const [prompt, setPrompt] = useState("");
  const [isPending, setIsPending] = useState(false);
  const [hasError, setHasError] = useState(false);
  const canSubmit = prompt.trim() !== "" && !isPending;

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const normalizedPrompt = prompt.trim();
    if (isPending || normalizedPrompt === "") {
      return;
    }

    setHasError(false);
    setIsPending(true);
    try {
      const conversationKey = crypto.randomUUID();
      const messageKey = crypto.randomUUID();
      const conversationResponse = await webBrowserMutation("/web/v1/conversations", {
        method: "POST",
        headers: { "X-Idempotency-Key": conversationKey },
      });
      if (conversationResponse.status !== 200 && conversationResponse.status !== 201) {
        throw new Error("Unable to complete the request.");
      }
      const conversation = parseConversationList({ items: [await conversationResponse.json()] }).items[0];
      if (conversation === undefined) {
        throw new Error("Unable to complete the request.");
      }

      const messageResponse = await webBrowserMutation(`/web/v1/conversations/${conversation.id}/messages`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-Idempotency-Key": messageKey,
        },
        body: JSON.stringify({ prompt: normalizedPrompt }),
      });
      if (messageResponse.status !== 200 && messageResponse.status !== 201) {
        throw new Error("Unable to complete the request.");
      }
      parseWebChatJob(await messageResponse.json());
      router.push(`/app/chat/${conversation.id}`);
    } catch {
      setHasError(true);
    } finally {
      setIsPending(false);
    }
  };

  return (
    <form className={styles.form} onSubmit={(event) => void submit(event)}>
      <label className={styles.promptField}>
        <span>{ru.workspace.promptLabel}</span>
        <textarea
          disabled={isPending}
          onChange={(event) => setPrompt(event.target.value)}
          placeholder={ru.workspace.promptPlaceholder}
          rows={5}
          value={prompt}
        />
      </label>
      <div className={styles.actions}>
        <Button disabled={!canSubmit} type="submit">
          {isPending ? ru.workspace.promptPending : ru.workspace.promptSubmit}
        </Button>
        <p>{ru.workspace.promptSupport}</p>
      </div>
      {hasError ? (
        <p className={styles.error} role="alert">
          {ru.workspace.promptFailure}
        </p>
      ) : null}
    </form>
  );
}
