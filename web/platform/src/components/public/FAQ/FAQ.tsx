import styles from "./FAQ.module.css";

export type FAQItem = {
  answer: string;
  question: string;
};

type FAQProps = {
  items: readonly FAQItem[];
};

export function FAQ({ items }: FAQProps) {
  return (
    <div className={styles.list}>
      {items.map((item) => (
        <details key={item.question}>
          <summary>{item.question}<span aria-hidden="true">⌄</span></summary>
          <p>{item.answer}</p>
        </details>
      ))}
    </div>
  );
}
