"use client";

import { useEffect, useId, useRef, useState } from "react";

import { ModalBackdrop } from "@/components/ui/ModalBackdrop/ModalBackdrop";
import selectableStyles from "@/components/ui/selectable-control.module.css";

import styles from "./TokenTopUpDialog.module.css";

type TokenTopUpDialogProps = {
  onClose: () => void;
};

type TokenPackage = {
  discount?: string;
  id: string;
  previousPrice?: string;
  price: string;
  tokenCount: string;
};

const tokenPackages: TokenPackage[] = [
  { id: "tokens-20000", tokenCount: "20 000", price: "9 200", previousPrice: "10 575", discount: "-13%" },
  { id: "tokens-10000", tokenCount: "10 000", price: "4 700" },
  { id: "tokens-3000", tokenCount: "3 000", price: "1 500" },
  { id: "tokens-1500", tokenCount: "1 500", price: "750" },
  { id: "tokens-800", tokenCount: "800", price: "400" },
];

function CloseIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24">
      <path d="m5 5 14 14M19 5 5 19" fill="none" stroke="currentColor" strokeLinecap="round" strokeWidth="1.7" />
    </svg>
  );
}

function TokenPackageMark({ index }: Readonly<{ index: number }>) {
  return (
    <span aria-hidden="true" className={styles.packageMark} data-size={index}>
      <span />
      <span />
      <span />
    </span>
  );
}

export function TokenTopUpDialog({ onClose }: Readonly<TokenTopUpDialogProps>) {
  const [selectedPackageId, setSelectedPackageId] = useState(tokenPackages[0].id);
  const closeButtonRef = useRef<HTMLButtonElement>(null);
  const titleId = useId();
  const selectedPackage = tokenPackages.find((tokenPackage) => tokenPackage.id === selectedPackageId)
    ?? tokenPackages[0];

  useEffect(() => {
    const previouslyFocusedElement = document.activeElement instanceof HTMLElement
      ? document.activeElement
      : null;
    closeButtonRef.current?.focus();

    return () => previouslyFocusedElement?.focus();
  }, []);

  return (
    <ModalBackdrop onClose={onClose} testId="token-top-up-backdrop">
      {(requestClose) => (
        <section
          aria-labelledby={titleId}
          aria-modal="true"
          className={styles.dialog}
          role="dialog"
        >
          <header className={styles.header}>
            <div>
              <h2 id={titleId}>Пополнить баланс токенов</h2>
              <p>Выберите подходящий пакет токенов.</p>
            </div>
            <button
              aria-label="Закрыть пополнение баланса"
              className={styles.closeButton}
              onClick={requestClose}
              ref={closeButtonRef}
              type="button"
            >
              <CloseIcon />
            </button>
          </header>

          <fieldset aria-label="Пакеты токенов" className={styles.packageList}>
            {tokenPackages.map((tokenPackage, index) => {
              const isSelected = tokenPackage.id === selectedPackageId;
              const optionLabel = `${tokenPackage.tokenCount} токенов за ${tokenPackage.price} ₽`;

              return (
                <label className={`${selectableStyles.control} ${styles.packageCard}`} key={tokenPackage.id}>
                  <input
                    aria-label={optionLabel}
                    checked={isSelected}
                    className={styles.radioInput}
                    name="token-package"
                    onChange={() => setSelectedPackageId(tokenPackage.id)}
                    type="radio"
                    value={tokenPackage.id}
                  />
                  <TokenPackageMark index={index} />
                  <span className={styles.packageDetails}>
                    <span className={styles.tokenLine}>
                      <strong>{tokenPackage.tokenCount}</strong>
                      <em>токенов</em>
                    </span>
                    <span className={styles.priceLine}>
                      <span>за {tokenPackage.price} ₽</span>
                      {tokenPackage.previousPrice ? <s>{tokenPackage.previousPrice} ₽</s> : null}
                      {tokenPackage.discount ? <mark>{tokenPackage.discount}</mark> : null}
                    </span>
                  </span>
                </label>
              );
            })}
          </fieldset>

          <button className={styles.purchaseButton} type="button">
            Купить за {selectedPackage.price} ₽
          </button>
        </section>
      )}
    </ModalBackdrop>
  );
}
