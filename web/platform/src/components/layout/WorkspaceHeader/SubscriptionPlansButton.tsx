"use client";

import { useMessages } from "@/i18n/LocaleProvider";


import { useCallback, useState } from "react";
import { SubscriptionPlansDialog } from "./SubscriptionPlansDialog";

type SubscriptionPlansButtonProps = {
  className?: string;
};

export function SubscriptionPlansButton({ className }: SubscriptionPlansButtonProps) {
  const msg = useMessages();
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
        {msg("subscriptionPlansButton.chooseAPlan")}</button>
      {plansAreOpen ? <SubscriptionPlansDialog onClose={closePlans} /> : null}
    </>
  );
}
