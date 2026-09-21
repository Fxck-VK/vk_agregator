"use client";

import { useDictionary } from "@/i18n/LocaleProvider";


import { AccountMenu } from "@/features/account/AccountMenu/AccountMenu";
import { useWorkspaceLogout } from "@/features/session/WorkspaceLogout/WorkspaceLogoutBoundary";
import type { AccountProfile } from "@/lib/web-api/contracts";

import styles from "./AccountControl.module.css";

type AccountControlProps = {
  profile: AccountProfile;
};

export function AccountControl({ profile }: AccountControlProps) {
  const t = useDictionary();
  const { logout } = useWorkspaceLogout();
  const identity = profile.identity_refs.find((candidate) => candidate.verified && candidate.label.trim() !== "");
  const label = identity?.label.trim();

  return (
    <div className={styles.control}>
      <AccountMenu
        identityLabel={label ?? t.account.unavailableLabel}
        isLogoutPending={false}
        onLogout={logout}
      />
    </div>
  );
}
