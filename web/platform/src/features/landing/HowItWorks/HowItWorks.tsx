import Image from "next/image";

import styles from "./HowItWorks.module.css";

const steps = [
  ["1", "Выберите инструмент", "Откройте чат, генератор или конкретную модель из каталога."],
  ["2", "Опишите задачу", "Введите запрос, приложите материал и настройте результат, если это необходимо."],
  ["3", "Получите результат", "Следите за статусом и находите готовые материалы в своей библиотеке файлов."],
] as const;

export function HowItWorks() {
  return (
    <div className={styles.layout}>
      <div>
        <p className={styles.eyebrow}>Три понятных шага</p>
        <h2 id="how-title">Как работает NeiroHub</h2>
        <ol className={styles.steps}>
          {steps.map(([number, title, description]) => (
            <li key={number}><span>{number}</span><div><h3>{title}</h3><p>{description}</p></div></li>
          ))}
        </ol>
      </div>
      <div className={styles.poster}>
        <Image
          alt="Интерфейс NeiroHub с выбором нейросети и полем запроса"
          fill
          loading="lazy"
          sizes="(max-width: 880px) 100vw, 44vw"
          src="/inspiration/paper-crane-cloud.png"
        />
        <span>Выберите → Опишите → Получите</span>
      </div>
    </div>
  );
}
