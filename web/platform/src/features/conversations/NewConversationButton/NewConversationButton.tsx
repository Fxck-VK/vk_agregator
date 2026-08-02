"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";

import { Button } from "@/components/ui/Button/Button";
import { ru } from "@/i18n/ru";
import { webBrowserMutation } from "@/lib/web-api/browser";
import { parseConversationList } from "@/lib/web-api/contracts";

import styles from "./NewConversationButton.module.css";

export function NewConversationButton() {
  const router = useRouter();
  const [isPending, setIsPending] = useState(false);
  const [hasError, setHasError] = useState(false);

  const createConversation = async () => {
    if (isPending) {
      return;
    }

    setHasError(false);
    setIsPending(true);

    try {
      const idempotencyKey = crypto.randomUUID();
      const response = await webBrowserMutation("/web/v1/conversations", {
        method: "POST",
        headers: { "X-Idempotency-Key": idempotencyKey },
      });
      if (response.status !== 200 && response.status !== 201) {
        throw new Error("Unable to complete the request.");
      }

      const conversation = parseConversationList({ items: [await response.json()] }).items[0];
      router.refresh();
      router.push("/app/chat/" + conversation.id);
    } catch {
      setHasError(true);
    } finally {
      setIsPending(false);
    }
  };

  return (
    <div className={styles.create}>
      <Button disabled={isPending} onClick={createConversation}>
        {isPending ? ru.conversations.createPending : ru.conversations.createLabel}
      </Button>
      {hasError ? (
        <p className={styles.error} role="alert">
          {ru.conversations.createFailure}
        </p>
      ) : null}
    </div>
  );
}
