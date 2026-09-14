"use client";

import { useEffect, useRef } from "react";

import { ModalBackdrop } from "@/components/ui/ModalBackdrop/ModalBackdrop";
import { ModalCloseButton } from "@/components/ui/ModalCloseButton/ModalCloseButton";
import { ScrollArea } from "@/components/ui/ScrollArea/ScrollArea";

import styles from "./SubscriptionPlansDialog.module.css";

type SubscriptionPlansDialogProps = {
  onClose: () => void;
};

type Plan = {
  name: string;
  price: string;
  period: string;
  previousPrice?: string;
  discount?: string;
  description: string;
  creditPeriod: string;
  credits: string;
  features: string[];
  popular?: boolean;
};

const plans: Plan[] = [
  {
    name: "Lite",
    price: "199",
    period: "₽/нед",
    description: "Пробный тариф",
    creditPeriod: "Разово",
    credits: "400",
    features: [
      "До 13 генераций изображений: Nano Banana, GPT Image 2 и другие модели",
      "До 2 генераций видео: генератор видео, Google Veo и Kling",
      "Доступ к популярным нейросетям: ChatGPT, Gemini, Claude и другим",
    ],
  },
  {
    name: "Start+",
    price: "549",
    period: "₽/мес",
    previousPrice: "632",
    discount: "−15%",
    description: "Идеально для старта",
    creditPeriod: "Каждый месяц",
    credits: "1250",
    features: [
      "До 41 генерации изображений в популярных моделях",
      "До 15 генераций видео",
      "Доступ к ChatGPT, Gemini, Claude, Suno, Kimi и Qwen",
      "4 генерации презентаций",
    ],
  },
  {
    name: "Pro",
    price: "999",
    period: "₽/мес",
    previousPrice: "1200",
    discount: "−20%",
    description: "Оптимальный выбор",
    creditPeriod: "Каждый месяц",
    credits: "2250",
    features: [
      "До 75 генераций изображений в популярных моделях",
      "До 15 генераций видео",
      "Доступ ко всем основным нейросетям",
      "7 генераций презентаций",
    ],
  },
  {
    name: "Ultima",
    price: "1999",
    period: "₽/мес",
    previousPrice: "2600",
    discount: "−30%",
    description: "Больше возможностей",
    creditPeriod: "Каждый месяц",
    credits: "4800",
    features: [
      "До 160 генераций изображений",
      "До 32 генераций видео",
      "Расширенный доступ к нейросетям",
      "16 генераций презентаций",
    ],
    popular: true,
  },
  {
    name: "Elite",
    price: "4999",
    period: "₽/мес",
    previousPrice: "7000",
    discount: "−40%",
    description: "Максимум пользы",
    creditPeriod: "Каждый месяц",
    credits: "12000",
    features: [
      "До 408 генераций изображений",
      "До 62 генераций видео",
      "Максимальные лимиты для всех нейросетей",
      "41 генерация презентаций",
    ],
  },
];

const teamFeatures = [
  ["Индивидуальные лимиты", "Гибкая настройка под ваши задачи"],
  ["Приоритетная скорость", "Выделенные мощности без очередей"],
  ["Командная работа", "Общие пространства и управление доступами"],
  ["Персональная поддержка", "Менеджер и быстрый саппорт"],
] as const;

function PlanCard({ plan }: { plan: Plan }) {
  return (
    <article className={`${styles.planCard} ${plan.popular ? styles.popularPlan : ""}`}>
      {plan.popular ? <span className={styles.popularLabel}>Популярное ✦</span> : null}
      {plan.discount ? <span className={styles.discount}>{plan.discount}</span> : null}

      <div className={styles.planSummary}>
        <h3>{plan.name}</h3>
        <p className={styles.price}>
          <strong>{plan.price}</strong> <span>{plan.period}</span>{" "}
          {plan.previousPrice ? <del>{plan.previousPrice}</del> : null}
        </p>
        <p className={styles.description}>{plan.description}</p>
        <button className={styles.subscribeButton} type="button">
          Активировать подписку
        </button>
      </div>

      <div className={styles.creditRow}>
        <span>{plan.creditPeriod}:</span>
        <strong><span aria-hidden="true">✦</span> {plan.credits}</strong>
      </div>

      <ul className={styles.featureList}>
        {plan.features.map((feature) => (
          <li key={feature}><span aria-hidden="true">✓</span><span>{feature}</span></li>
        ))}
      </ul>
    </article>
  );
}

export function SubscriptionPlansDialog({ onClose }: SubscriptionPlansDialogProps) {
  const closeButtonRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    const previouslyFocusedElement = document.activeElement instanceof HTMLElement
      ? document.activeElement
      : null;
    closeButtonRef.current?.focus();

    return () => {
      previouslyFocusedElement?.focus();
    };
  }, []);

  return (
    <ModalBackdrop onClose={onClose} testId="subscription-plans-backdrop">
      {(requestClose) => <section
        aria-labelledby="subscription-plans-title"
        aria-modal="true"
        className={styles.dialog}
        role="dialog"
      >
        <header className={styles.dialogHeader}>
          <div>
            <h2 id="subscription-plans-title">С подпиской — максимум возможностей</h2>
            <p>Выберите подходящий объём возможностей NeiroHub</p>
          </div>
          <ModalCloseButton
            aria-label="Закрыть тарифы"
            className={styles.closeButtonPlacement}
            onClick={requestClose}
            ref={closeButtonRef}
          />
          <button className={styles.promoButton} type="button">Активировать промокод</button>
        </header>

        <ScrollArea className={styles.scrollArea} viewportClassName={styles.scrollViewport}>
          <div className={styles.planGrid}>
            {plans.map((plan) => <PlanCard key={plan.name} plan={plan} />)}

            <article className={`${styles.planCard} ${styles.teamPlan}`}>
              <div className={styles.planSummary}>
                <h3>Командный тариф</h3>
                <p className={styles.description}>Корпоративный тариф с индивидуальной стоимостью для компаний</p>
                <button className={styles.managerButton} type="button">Связаться с менеджером</button>
              </div>
              <ul className={styles.teamFeatureList}>
                {teamFeatures.map(([title, description]) => (
                  <li key={title}>
                    <span aria-hidden="true">✓</span>
                    <span><strong>{title}</strong><small>{description}</small></span>
                  </li>
                ))}
              </ul>
            </article>
          </div>

          <footer className={styles.dialogFooter}>
            <p>Покупая подписку, вы соглашаетесь с пользовательским соглашением и рекуррентными платежами.</p>
            <button type="button">Подробнее о тарифах</button>
          </footer>
        </ScrollArea>
      </section>}
    </ModalBackdrop>
  );
}
