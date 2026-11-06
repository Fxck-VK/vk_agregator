"use client";
import { useId, useRef, useState } from "react";
import { Button } from "@/components/ui/Button/Button";
import { ModeSwitchPanel } from "@/components/ui/ModeSwitchPanel/ModeSwitchPanel";
import { ScrollArea } from "@/components/ui/ScrollArea/ScrollArea";
import { StateNotice } from "@/components/ui/AsyncState/AsyncState";
import { useMessages } from "@/i18n/LocaleProvider";
import { useRouter } from "@/i18n/navigation";
import type { MessageKey } from "@/i18n/messages";
import { allowsReleasePath, releaseFeatures, releasePages, type ReleaseView } from "./policy";
import { useReleaseView } from "./ReleaseViewProvider";
import styles from "./ReleaseReadinessPanel.module.css";

const readinessLabels = { ready: "releasePreview.ready", "needs-check": "releasePreview.needsCheck", "not-connected": "releasePreview.notConnected" } as const;
const publicationLabels: Readonly<Record<string, MessageKey>> = {
  draft: "releasePreview.draft", published: "releasePreview.published", scheduled: "releasePreview.scheduled", archived: "releasePreview.archived",
};

export function ReleaseReadinessPanel() {
  const msg = useMessages();
  const router = useRouter();
  const release = useReleaseView();
  const titleId = useId();
  const activeRequest = useRef(false);
  const [pending, setPending] = useState(false);
  const [failed, setFailed] = useState(false);
  const busy = pending || release.switching;

  const changeView = async (view: ReleaseView, path?: string) => {
    if (activeRequest.current || busy || view !== release.view && !release.canSwitch) return;
    activeRequest.current = true;
    setPending(true);
    setFailed(false);
    try {
      if (view !== release.view && !await release.changeView(view)) { setFailed(true); return; }
      if (path) router.push(path);
    } catch { setFailed(true); }
    finally { activeRequest.current = false; setPending(false); }
  };

  const openButton = (name: string, path: string, production: boolean) => {
    const requiresDev = release.view === "prod" && (!production || !release.allowsPath(path));
    return <Button variant="outline" className={styles.openButton}
      aria-label={msg(requiresDev ? "releasePreview.openDevLabel" : "releasePreview.openLabel", { name })}
      disabled={busy || requiresDev && !release.canSwitch}
      onClick={() => void changeView(requiresDev ? "dev" : release.view, path)}>
      {msg(requiresDev ? "releasePreview.openDev" : "releasePreview.open")}
    </Button>;
  };

  return <section className={styles.panel} aria-labelledby={titleId} data-ui="release-readiness-panel">
    <h3 id={titleId} className={styles.title}>{msg("releasePreview.title")}</h3>
    <ModeSwitchPanel fullWidth density="compact" activeID={release.view} ariaLabel={msg("releasePreview.view")}
      items={([
        { id: "prod", label: msg("releasePreview.prod") },
        { id: "dev", label: msg("releasePreview.dev") },
      ] as const).map(item => ({ ...item, disabled: busy || !release.canSwitch }))}
      onChange={view => { if (view !== release.view) void changeView(view); }} />
    <p className={styles.note}>{msg("releasePreview.description")}</p>
    {!release.canSwitch ? <p className={styles.note}>{msg("releasePreview.readOnly")}</p> : null}
    {busy ? <StateNotice inline kind="loading">{msg("releasePreview.busy")}</StateNotice> : null}
    {failed || release.error ? <StateNotice inline kind="error">{msg("releasePreview.error")}</StateNotice> : null}
    <ScrollArea className={styles.inventory} viewportClassName={styles.viewport}
      viewportProps={{ role: "region", "aria-label": msg("releasePreview.inventory"), tabIndex: 0 }}>
      <h4 className={styles.groupTitle}>{msg("releasePreview.features", { count: releaseFeatures.length })}</h4>
      <ul className={styles.list}>
        {releaseFeatures.map(feature => <li className={styles.card} key={feature.id} data-release-feature={feature.id}>
          <details>
            <summary className={styles.summary}><span>{feature.name}</span><span className={styles.status}>{msg(readinessLabels[feature.status])}</span></summary>
            <dl className={styles.details}>
              <dt>{msg("releasePreview.works")}</dt><dd>{feature.works}</dd>
              <dt>{msg("releasePreview.remaining")}</dt><dd>{feature.remaining}</dd>
              <dt>{msg("releasePreview.verification")}</dt><dd>{feature.verification}</dd>
            </dl>
          </details>
          <div className={styles.actions}><span className={styles.note}>{msg(feature.production ? "releasePreview.included" : "releasePreview.devOnly")}</span>{openButton(feature.name, feature.path, feature.production)}</div>
        </li>)}
      </ul>
      <h4 className={styles.groupTitle}>{msg("releasePreview.pages", { count: releasePages.length })}</h4>
      <p className={styles.note}>{msg("releasePreview.pagesNote")}</p>
      <ul className={styles.list}>
        {releasePages.map(page => {
          const production = allowsReleasePath(page.path, "prod");
          return <li className={styles.card} key={page.id} data-release-page={page.id}>
            <strong className={styles.pageTitle}>{page.title}</strong>
            <span className={styles.note}>{page.template} · {msg(publicationLabels[page.status] ?? "releasePreview.unknown")}</span>
            <div className={styles.actions}><span className={styles.note}>{msg(production ? "releasePreview.included" : "releasePreview.devOnly")}</span>{openButton(page.title, page.path, production)}</div>
          </li>;
        })}
      </ul>
    </ScrollArea>
  </section>;
}
