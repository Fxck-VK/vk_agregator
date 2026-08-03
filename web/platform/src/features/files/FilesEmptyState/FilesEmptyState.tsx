import styles from "./FilesEmptyState.module.css";

type FilesEmptyStateProps = {
  description: string;
  title: string;
};

export function FilesEmptyState({ description, title }: Readonly<FilesEmptyStateProps>) {
  return (
    <section className={styles.emptyState}>
      <svg aria-hidden="true" className={styles.folder} focusable="false" viewBox="0 0 112 88">
        <path d="M8 26c0-6.627 5.373-12 12-12h27l10 10h35c6.627 0 12 5.373 12 12v34c0 6.627-5.373 12-12 12H20c-6.627 0-12-5.373-12-12V26Z" />
        <path d="M8 36c0-6.627 5.373-12 12-12h72c6.627 0 12 5.373 12 12v34c0 6.627-5.373 12-12 12H20c-6.627 0-12-5.373-12-12V36Z" />
      </svg>
      <div>
        <h2>{title}</h2>
        <p>{description}</p>
      </div>
    </section>
  );
}
