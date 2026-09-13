"use client";

import { useCallback, useState } from "react";
import { SubscriptionPlansDialog } from "./SubscriptionPlansDialog";

type SubscriptionPlansButtonProps = {
  className?: string;
};

export function SubscriptionPlansButton({ className }: SubscriptionPlansButtonProps) {
  const [plansAreOpen, setPlansAreOpen] = useState(false);
  const closePlans = useCallback(() => setPlansAreOpen(false), []);

  return (
    <>
      <button
        aria-expanded={plansAreOpen}
        aria-haspopup="dialog"
        className={className}
        onClick={() => setPlansAreOpen(true)}
        type="button"
      >
        Выбрать тариф
      </button>
      {plansAreOpen ? <SubscriptionPlansDialog onClose={closePlans} /> : null}
    </>
  );
}
