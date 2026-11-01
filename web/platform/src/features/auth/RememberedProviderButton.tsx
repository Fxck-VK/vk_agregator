"use client";

import { type ReactNode, useId, useRef, useState } from "react";
import { assetPaths } from "@/assets/asset-paths";
import { AssetIcon } from "@/components/icons/AssetIcon";
import { Button } from "@/components/ui/Button/Button";
import { PopoverPanel } from "@/components/ui/PopoverPanel/PopoverPanel";
import selectableStyles from "@/components/ui/selectable-control.module.css";
import { useDictionary } from "@/i18n/LocaleProvider";
import styles from "./RememberedProviderButton.module.css";

type Props = {
  label: string;
  caption: string;
  providerIcon: ReactNode;
  pending: boolean;
  preview: boolean;
  onLogin: () => void;
  onForget: () => void;
};

/** One provider row, with an independent account menu; the hint grants no access. */
export function RememberedProviderButton({ label, caption, providerIcon, pending, preview, onLogin, onForget }: Readonly<Props>) {
  const { auth: t } = useDictionary();
  const [open, setOpen] = useState(false);
  const anchor = useRef<HTMLButtonElement>(null);
  const menuID = useId();
  return <div className={styles.row}>
    <Button className={styles.login} disabled={pending || preview} onClick={() => { setOpen(false); onLogin(); }}>
      <AssetIcon className={styles.avatar} iconName="account-avatar" source={assetPaths.icons.accountMenu.avatar} />
      <span className={styles.copy}>
        <span className={styles.name} title={label}>{t.continueAs} {label}</span>
        <small className={styles.caption}>{caption}</small>
      </span>
      {providerIcon}
    </Button>
    <button className={styles.accountMenu} ref={anchor} type="button" aria-label={t.chooseAccount} aria-haspopup="menu" aria-expanded={open} aria-controls={open ? menuID : undefined} disabled={pending} onClick={() => setOpen(value => !value)}>
      <svg width="12" height="12" viewBox="0 0 16 16" fill="none" aria-hidden="true"><path d="m4 6 4 4 4-4" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" /></svg>
    </button>
    <PopoverPanel anchorRef={anchor} id={menuID} isOpen={open} label={t.chooseAccount} role="menu" itemVariant="action" width={240} onClose={() => setOpen(false)}>
      <button className={`${selectableStyles.control} ${styles.menuAction}`} role="menuitem" type="button" disabled={pending} onClick={() => { setOpen(false); onForget(); }}>{t.useAnotherAccount}</button>
    </PopoverPanel>
  </div>;
}
