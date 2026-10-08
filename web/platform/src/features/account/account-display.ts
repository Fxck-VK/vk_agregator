import type { Dictionary } from "@/i18n/dictionary";
import type { AccountProfile } from "@/lib/web-api/contracts";

const disabledIdentityProviders = new Set(["google"]);

type AccountIdentityRef = AccountProfile["identity_refs"][number];

export type AccountDisplayIdentity = {
  hasVerifiedIdentity: boolean;
  label: string;
};

export function isDisabledAccountProvider(provider: string): boolean {
  return disabledIdentityProviders.has(provider);
}

export function isUsableAccountLoginIdentity(identity: AccountIdentityRef): boolean {
  return identity.verified && identity.provider !== "phone" && !isDisabledAccountProvider(identity.provider);
}

function hasSafeLabel(identity: AccountIdentityRef): boolean {
  return identity.verified && identity.label.trim() !== "";
}

export function getAccountDisplayIdentity(identityRefs: AccountProfile["identity_refs"], t: Dictionary): AccountDisplayIdentity {
  const primaryEmail = identityRefs.find((identity) => (
    hasSafeLabel(identity)
    && identity.provider === "email"
    && identity.email_role === "primary"
  ));
  const firstEmail = identityRefs.find((identity) => hasSafeLabel(identity) && identity.provider === "email");
  const firstUsableIdentity = identityRefs.find((identity) => hasSafeLabel(identity) && isUsableAccountLoginIdentity(identity));
  const identity = primaryEmail ?? firstEmail ?? firstUsableIdentity;

  return {
    hasVerifiedIdentity: identity !== undefined,
    label: identity?.label.trim() ?? t.account.myProfileLabel,
  };
}
