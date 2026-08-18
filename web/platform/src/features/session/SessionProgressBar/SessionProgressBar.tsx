import styles from "./SessionProgressBar.module.css";

type SessionProgressBarProps = {
  label: string;
  visible: boolean;
};

export function SessionProgressBar({
  label,
  visible,
}: SessionProgressBarProps) {
  if (!visible) {
    return null;
  }

  return (
    <div aria-label={label} className={styles.track} role="progressbar">
      <span aria-hidden="true" className={styles.indicator} />
    </div>
  );
}
