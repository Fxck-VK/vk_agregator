"use client";

import { useMessages } from "@/i18n/LocaleProvider";


import { useCallback, useState } from "react";

import { CreditAmount, getCreditAmountLabel } from "@/components/ui/CreditAmount/CreditAmount";

import { TokenTopUpDialog } from "./TokenTopUpDialog";

type BalanceTopUpButtonProps = {
  balance: number;
  className?: string;
};

export function BalanceTopUpButton({ balance, className }: Readonly<BalanceTopUpButtonProps>) {
  const msg = useMessages();
  const [dialogIsOpen, setDialogIsOpen] = useState(false);
  const closeDialog = useCallback(() => setDialogIsOpen(false), []);
  const accessibleLabel = msg("balanceTopUpButton.topUpTokensCurrentBalanceValue", { value1: getCreditAmountLabel(balance, undefined, msg) });

  return (
    <>
      <button
        aria-expanded={dialogIsOpen}
        aria-haspopup="dialog"
        aria-label={accessibleLabel}
        className={className}
        data-testid="workspace-balance"
        onClick={() => setDialogIsOpen(true)}
        type="button"
      >
        <CreditAmount aria-hidden="true" value={balance} />
      </button>
      {dialogIsOpen ? <TokenTopUpDialog onClose={closeDialog} /> : null}
    </>
  );
}
