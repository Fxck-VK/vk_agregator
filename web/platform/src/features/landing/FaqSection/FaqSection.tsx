import { landingFaq } from "../landing-content";
import styles from "./FaqSection.module.css";

export function FaqSection() {
  return (
    <div>
      <div className={styles.heading}><p>Ответы на вопросы</p><h2 id="faq-title">Частые вопросы</h2></div>
      <div className={styles.list}>
        {landingFaq.map((item, index) => (
          <details className={styles.item} key={item.id} name="landing-faq" open={index === 0}>
            <summary>
              <span>{item.question}</span><span aria-hidden="true">+</span>
            </summary>
            <div className={styles.answer}><p>{item.answer}</p></div>
          </details>
        ))}
      </div>
    </div>
  );
}
