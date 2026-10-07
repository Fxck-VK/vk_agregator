import type { ReactNode } from "react";

import { CreditAmount } from "@/components/ui/CreditAmount/CreditAmount";
import { useMessages } from "@/i18n/LocaleProvider";
import type { MessageKey } from "@/i18n/messages";

import type { MusicOperationID, MusicWorkspaceOperation } from "./MusicWorkspace";
import styles from "./MusicWorkspace.module.css";

export function MusicOperationPrice({ operation }: Readonly<{ operation?: MusicWorkspaceOperation }>) {
  const msg = useMessages();
  if (operation?.quote) return <CreditAmount value={operation.quote.credits} />;
  const value = operation?.maxEstimateCredits ?? operation?.estimateCredits;
  if (value == null) return null;
  return <CreditAmount prefix={msg(operation?.maxEstimateCredits != null ? "music.operation.upTo" : "music.studio.approximatePrice")} value={value} />;
}

const descriptions: Partial<Record<MusicOperationID, MessageKey>> = {
  lyrics: "music.studio.lyricsDescription",
  sounds: "music.studio.soundsDescription",
  cover: "music.studio.coverDescription",
  upload_cover: "music.studio.uploadCoverDescription",
  upload_extend: "music.studio.uploadExtendDescription",
  mashup: "music.studio.mashupDescription",
  stems: "music.studio.stemsDescription",
  stems_all: "music.studio.stemsAllDescription",
  add_vocals: "music.studio.vocalsDescription",
  add_instrumental: "music.studio.instrumentalDescription",
};

export function MusicToolIcon({ kind = "music" }: Readonly<{ kind?: string }>) {
  const paths: Record<string, ReactNode> = {
    folder: <path d="M3 7V5a1 1 0 0 1 1-1h5l2 3h9a1 1 0 0 1 1 1v11a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1Z" />,
    lyrics: <><path d="M5 5h14M5 10h14M5 15h9M5 20h6" /></>,
    settings: <><path d="M4 6h16M4 12h16M4 18h16" /><path d="M9 3v6m6 0v6m-6 0v6" /></>,
    cover: <><path d="M20 7v5h-5M4 17v-5h5" /><path d="M6 7a7 7 0 0 1 12-1l2 3M4 15l2 3a7 7 0 0 0 12-1" /></>,
    voice: <><rect x="9" y="3" width="6" height="12" rx="3" /><path d="M6 11v1a6 6 0 0 0 12 0v-1M12 18v3m-3 0h6" /></>,
    waves: <path d="M3 10v4m4-7v10m5-14v18m5-14v10m4-7v4" />,
    music: <><path d="M9 17V5l11-2v12M9 8l11-2" /><ellipse cx="6" cy="18" rx="3" ry="3" /><ellipse cx="17" cy="16" rx="3" ry="3" /></>,
  };
  const icon = kind === "upload_cover" ? "cover"
    : ["stems", "stems_all", "sounds", "instrumental"].includes(kind) ? "waves"
      : kind === "add_vocals" ? "voice"
        : kind === "mashup" ? "settings" : kind;
  return <svg aria-hidden="true" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">{paths[icon] ?? paths.music}</svg>;
}

export function MusicToolCard({ active, busy, children, description, icon, label, onClick, operation, unavailable }: Readonly<{
  active?: boolean;
  busy: boolean;
  children?: ReactNode;
  description?: string;
  icon?: string;
  label: string;
  onClick: () => void;
  operation?: MusicWorkspaceOperation;
  unavailable?: boolean;
}>) {
  const msg = useMessages();
  const descriptionKey = operation ? descriptions[operation.id] : undefined;
  return (
    <button aria-label={label} aria-pressed={active === true} className={styles.toolCard} disabled={busy} onClick={onClick} type="button">
      <span className={styles.toolCardHeader}>
        <span className={styles.toolIcon}><MusicToolIcon kind={icon ?? operation?.id} /></span>
        <span className={styles.toolName}>{label}</span>
      </span>
      <span className={styles.toolDescription}>{description ?? (descriptionKey ? msg(descriptionKey) : operation?.description)}</span>
      <span className={styles.toolCardFooter}>
        <span className={styles.toolTag}>{unavailable ? msg("music.studio.toolUnavailable") : children ?? msg("music.studio.openTool")}</span>
        <span className={styles.toolPrice}><MusicOperationPrice operation={operation} /></span>
      </span>
    </button>
  );
}
