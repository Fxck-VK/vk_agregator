import styles from "./AssistantTypingIndicator.module.css";

type AssistantTypingIndicatorProps = {
  label: string;
};

export function AssistantTypingIndicator({ label }: AssistantTypingIndicatorProps) {
  return (
    <span aria-label={label} aria-live="polite" className={styles.indicator} role="status">
      <span aria-hidden="true" className={styles.dot} />
      <span aria-hidden="true" className={styles.dot} />
      <span aria-hidden="true" className={styles.dot} />
    </span>
  );
}
