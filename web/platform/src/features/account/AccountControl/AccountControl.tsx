"use client";

import { useDictionary } from "@/i18n/LocaleProvider";


import { AccountMenu } from "@/features/account/AccountMenu/AccountMenu";
import { getAccountDisplayIdentity } from "@/features/account/account-display";
import { useWorkspaceLogout } from "@/features/session/WorkspaceLogout/WorkspaceLogoutBoundary";
import type { AccountProfile } from "@/lib/web-api/contracts";

import styles from "./AccountControl.module.css";

type AccountControlProps = {
  profile: AccountProfile;
};

export function AccountControl({ profile }: AccountControlProps) {
  const t = useDictionary();
  const { logout } = useWorkspaceLogout();
  const identity = getAccountDisplayIdentity(profile.identity_refs, t);

  return (
    <div className={styles.control}>
      <AccountMenu
        identityLabel={identity.label}
        isLogoutPending={false}
        onLogout={logout}
      />
    </div>
  );
}
