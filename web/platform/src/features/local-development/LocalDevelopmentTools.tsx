"use client";

import Link from "@/i18n/Link";
import { useEffect, useRef, useState, useSyncExternalStore } from "react";
import { createPortal } from "react-dom";
import { Button } from "@/components/ui/Button/Button";
import { ModalCloseButton } from "@/components/ui/ModalCloseButton/ModalCloseButton";
import { ModeSwitchPanel } from "@/components/ui/ModeSwitchPanel/ModeSwitchPanel";
import { PopoverPanel } from "@/components/ui/PopoverPanel/PopoverPanel";
import { Tooltip } from "@/components/ui/Tooltip/Tooltip";
import { useMessages } from "@/i18n/LocaleProvider";
import { registerDevelopmentTransport } from "@/lib/web-api/development-transport";
import { scenarioStore } from "./scenario-store";
import { pageChoiceStore } from "./page-choice-store";
import { createLocalSimulator } from "./simulator";
import type { LocalScenarios, UploadScenario } from "./scenarios";
import type { LocalConversationSeed } from "./conversation-simulator";
import { useReleaseView } from "@/features/release/ReleaseViewProvider";
import { ReleaseReadinessPanel } from "@/features/release/ReleaseReadinessPanel";
import styles from "./LocalDevelopmentTools.module.css";

const subscribeToHydration = () => () => {};

export function LocalDevelopmentTools({ seed }: { seed?: LocalConversationSeed }) {
  const msg = useMessages();
  const release = useReleaseView();
  const initialSeed = useRef(seed);
  const currentMessages = useRef(msg);
  useEffect(() => { currentMessages.current = msg; }, [msg]);
  const scenarios = useSyncExternalStore(scenarioStore.subscribe, scenarioStore.getSnapshot, scenarioStore.getServerSnapshot);
  const registeredChoice = useSyncExternalStore(pageChoiceStore.subscribe, pageChoiceStore.getSnapshot, pageChoiceStore.getServerSnapshot);
  const pageChoice = release.view === "dev" ? registeredChoice : null;
  const hydrated = useSyncExternalStore(subscribeToHydration, () => true, () => false);
  const anchor = useRef<HTMLButtonElement>(null);
  const [open, setOpen] = useState(false);
  useEffect(() => {
    if (process.env.NODE_ENV !== "development" || !release.localSimulation || !initialSeed.current) return;
    scenarioStore.initialize();
    const simulator = createLocalSimulator(scenarioStore.getSnapshot, { seed: initialSeed.current,
      reply: () => currentMessages.current("localDevelopment.reply"),
      imageReply: () => currentMessages.current("localDevelopment.imageReply") });
    const unregister = registerDevelopmentTransport(simulator.handle);
    return () => { unregister(); simulator.dispose(); };
  }, [release.localSimulation]);
  if (!release.canSwitch || !hydrated) return null;
  const choices: { id: UploadScenario; label: string }[] = [
    { id: "success", label: msg("localDevelopment.success") },
    { id: "error", label: msg("localDevelopment.error") },
    { id: "slow", label: msg("localDevelopment.slow") },
  ];
  const uploadChoices: { id: LocalScenarios["upload"]; label: string }[] = [
    ...choices, { id: "offline", label: msg("localDevelopment.offline") },
  ];
  const launcherLabel = msg("releasePreview.title");
  return createPortal(<>
    <div className={styles.launcher} data-ui="local-development-tools" data-open={open}>
      <Tooltip label={launcherLabel}>
        <Button ref={anchor} className={styles.launcherButton} variant="outline" aria-label={launcherLabel} aria-expanded={open} aria-haspopup="dialog" onClick={() => setOpen(!open)}>
          <svg className={styles.launcherIcon} aria-hidden="true" focusable="false" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
            <path d="M9 3h6M10 3v6l-5.7 9a2 2 0 0 0 1.7 3h12a2 2 0 0 0 1.7-3L14 9V3M7.5 14h9" />
            <path d="M10 17h.01M14 18h.01" />
          </svg>
        </Button>
      </Tooltip>
    </div>
    <PopoverPanel anchorRef={anchor} isOpen={open} onClose={() => setOpen(false)} label={msg("localDevelopment.title")} width={390} align="end" portalLayer={180}>
      <div className={styles.content}>
        <header className={styles.header}>
          <strong>{msg("localDevelopment.title")}</strong>
          <ModalCloseButton size="compact" aria-label={msg("localDevelopment.close")} onClick={() => { setOpen(false); anchor.current?.focus(); }} />
        </header>
        {pageChoice ? <>
          <section className={styles.pageChoice} aria-label={pageChoice.title} data-ui="development-page-choice">
            <strong>{pageChoice.title}</strong>
            <ModeSwitchPanel fullWidth density="compact" activeID={pageChoice.value} ariaLabel={pageChoice.title} items={pageChoice.options} onChange={pageChoice.onChange} />
            {pageChoice.description ? <p className={styles.note}>{pageChoice.description}</p> : null}
          </section>
          <details><summary>{launcherLabel}</summary><ReleaseReadinessPanel /></details>
        </> : <ReleaseReadinessPanel />}
        {release.localSimulation ? <details><summary>{msg("localDevelopment.title")}</summary>
        <p>{msg("localDevelopment.description")}</p>
        <Link href="/app/ui-states" onClick={() => setOpen(false)}>{msg("localDevelopment.states")}</Link>
        <Button variant="outline" aria-pressed={scenarios.enabled} onClick={() => scenarioStore.update({ ...scenarios, enabled: !scenarios.enabled })}>
          {msg(scenarios.enabled ? "localDevelopment.disable" : "localDevelopment.enable")}
        </Button>
        <strong>{msg("localDevelopment.upload")}</strong>
        <ModeSwitchPanel fullWidth activeID={scenarios.upload} ariaLabel={msg("localDevelopment.upload")} items={uploadChoices} onChange={upload => scenarioStore.update({ ...scenarios, enabled: true, upload })} />
        <strong>{msg("localDevelopment.quote")}</strong>
        <ModeSwitchPanel fullWidth activeID={scenarios.quote} ariaLabel={msg("localDevelopment.quote")} items={choices.filter(item => item.id !== "slow") as { id: "success" | "error"; label: string }[]} onChange={quote => scenarioStore.update({ ...scenarios, enabled: true, quote })} />
        <strong>{msg("localDevelopment.message")}</strong>
        <ModeSwitchPanel fullWidth activeID={scenarios.message} ariaLabel={msg("localDevelopment.message")} items={choices} onChange={message => scenarioStore.update({ ...scenarios, enabled: true, message })} />
        <p>{msg("localDevelopment.instructions")}</p>
        <p className={styles.note}>{msg("localDevelopment.scope")}</p>
        </details> : null}
      </div>
    </PopoverPanel>
  </>, document.body);
}
