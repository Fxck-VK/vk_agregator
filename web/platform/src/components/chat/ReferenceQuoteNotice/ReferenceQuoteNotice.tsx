"use client";

import { Button } from "@/components/ui/Button/Button";
import { PopoverSurface } from "@/components/ui/PopoverPanel/PopoverPanel";
import { useMessages } from "@/i18n/LocaleProvider";
import styles from "./ReferenceQuoteNotice.module.css";

export function ReferenceQuoteNotice({ message, onRetry }: { message: string | null; onRetry: () => void }) {
  const msg = useMessages();
  if (!message) return null;
  return (
    <PopoverSurface animated={false} className={styles.notice} viewportClassName={styles.content}>
      <p role="alert">{message}</p>
      <Button
        className={styles.retry}
        onClick={event => {
          event.currentTarget.closest("form")?.querySelector("textarea")?.focus({ preventScroll: true });
          onRetry();
        }}
        variant="outline"
      >
        {msg("referenceQuoteNotice.retry")}
      </Button>
    </PopoverSurface>
  );
}
