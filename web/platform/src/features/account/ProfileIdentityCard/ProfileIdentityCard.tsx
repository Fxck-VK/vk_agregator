"use client";

import { useEffect, useState } from "react";
import { CopyIcon } from "@/components/icons/CopyIcon";
import { Button } from "@/components/ui/Button/Button";
import { useDictionary } from "@/i18n/LocaleProvider";

import styles from "./ProfileIdentityCard.module.css";

type ProfileIdentityCardProps = {
  hasVerifiedIdentity: boolean;
  identityLabel: string;
  publicID?: string;
};

export function ProfileIdentityCard(props: ProfileIdentityCardProps) {
  return <IdentityCard key={props.publicID ?? ""} {...props} />;
}

function IdentityCard({ hasVerifiedIdentity, identityLabel, publicID }: ProfileIdentityCardProps) {
  const t = useDictionary();
  const [copyFeedback, setCopyFeedback] = useState<{ status: "pending" | "success" | "failure" } | null>(null);
  const copyStatus = copyFeedback?.status;
  useEffect(() => {
    if (copyFeedback?.status !== "success") return;
    const timer = setTimeout(() => setCopyFeedback(null), 5000);
    return () => clearTimeout(timer);
  }, [copyFeedback]);
  const copyID = async () => {
    if (!publicID || copyStatus === "pending") return;
    setCopyFeedback({ status: "pending" });
    try {
      await navigator.clipboard.writeText(publicID);
      setCopyFeedback({ status: "success" });
    } catch {
      setCopyFeedback({ status: "failure" });
    }
  };
  return (
    <section aria-label={t.profile.identityCardLabel} className={styles.card}>
      <span aria-hidden="true" className={styles.identityMark}>ID</span>
      <div className={styles.content}>
        <strong>{identityLabel}</strong>
        <p>{hasVerifiedIdentity ? t.profile.verifiedIdentity : t.profile.noVerifiedIdentity}</p>
        {publicID ? <>
          <div className={styles.publicIDRow}>
            <span className={styles.publicIDLabel}>{t.profile.publicIDLabel}</span>
            <span className={styles.publicIDValue}>{publicID}</span>
            <Button aria-label={t.profile.publicIDCopy} className={styles.copyButton}
              disabled={copyStatus === "pending"} onClick={() => void copyID()} variant="outline">
              <CopyIcon />
            </Button>
          </div>
          <p aria-live="polite" className={styles.copyStatus} role="status">
            {copyStatus === "pending" ? t.profile.publicIDCopyPending : copyStatus === "success" ? t.profile.publicIDCopied : copyStatus === "failure" ? t.profile.publicIDCopyFailure : null}
          </p>
        </> : null}
      </div>
    </section>
  );
}
