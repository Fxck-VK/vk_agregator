"use client";

import { useState, useSyncExternalStore } from "react";
import { ModelIcon } from "@/features/models/ModelIcon/ModelIcon";
import { getModelIconPath } from "@/features/models/model-icon-registry";
import { aiToolsEn as copy } from "@/i18n/public/ai-tools-en";
import Link from "@/i18n/Link";
import { OverviewHeading, OverviewTabs } from "./OverviewPrimitives";
import styles from "./ToolsOverview.module.css";
import {usePublicCopy} from "@/i18n/public/use-public-copy";


const wideQuery = "(min-width: 768px)";
const subscribe = (listener: () => void) => { const query = window.matchMedia?.(wideQuery); query?.addEventListener("change",listener); return () => query?.removeEventListener("change",listener); };
const wideSnapshot = () => window.matchMedia?.(wideQuery).matches ?? false;
const serverSnapshot = () => false;

export function OverviewModels() {
  const {copy} = usePublicCopy(publicCopySourcesForLocale);

  const [selected, setSelected] = useState<string>(copy.models.styles[0].id);
  const wide = useSyncExternalStore(subscribe, wideSnapshot, serverSnapshot);
  return <section id="models" className={styles.modelsSection}>
    <OverviewHeading title={copy.models.title} description={copy.models.description} />
    <div className={styles.modelBoard}>
      <OverviewTabs prefix="style" label={copy.controls.styles} items={copy.models.styles} selected={selected} onChange={setSelected} className={styles.styleTabs} orientation={wide ? "vertical" : "horizontal"} />
      <div>{copy.models.styles.map(style => <div id={`style-panel-${style.id}`} key={style.id} role="tabpanel" aria-labelledby={`style-tab-${style.id}`} hidden={selected !== style.id} tabIndex={0}>
        <h3 className={styles.styleTitle}>{style.label}</h3><p className={styles.styleDescription}>{style.description}</p>
        <div className={styles.modelGrid}>{style.models.map(model => <Link href={`/app?model=${encodeURIComponent(model.id)}`} className={styles.modelCard} key={model.id}>
          <span className={styles.modelTitle}><ModelIcon src={getModelIconPath(model.id)} className={styles.modelIcon} />{model.name}</span><span className={styles.modelDescription}>{model.description}</span><span className={styles.modelDot} aria-hidden="true" />
        </Link>)}</div>
      </div>)}</div>
    </div><p className={styles.modelNote}>{copy.models.note}</p>
  </section>;
}

const clients = copy.connections.clients.map((label,index) => ({ id: `client-${index}`, label }));
export function OverviewConnections() {
  const {clients,copy} = usePublicCopy(publicCopySourcesForLocale);

  const [selected,setSelected] = useState(clients[0].id);
  return <section id="connections" className={styles.connectionSection}>
    <OverviewHeading title={copy.connections.title} description={copy.connections.description}><Link className={styles.outlineAction} href="/mcp">{copy.connections.about}<span aria-hidden="true">↗</span></Link></OverviewHeading>
    <div className={styles.connectionBoard}>
      <OverviewTabs prefix="client" label={copy.controls.clients} items={clients} selected={selected} onChange={setSelected} className={styles.clientTabs} />
      {clients.map(client => <div key={client.id} id={`client-panel-${client.id}`} role="tabpanel" aria-labelledby={`client-tab-${client.id}`} hidden={selected !== client.id} className={styles.connectionPanel} tabIndex={0}>
        <div className={styles.connectionMeta}><span>{copy.connections.clientPrefix} {client.label}</span><span className={styles.comingSoon}>{copy.controls.comingSoon}</span></div>
        <ol>{copy.connections.steps.map(step => <li key={step}>{step}</li>)}</ol>
        <div className={styles.connectionEndpoint}><span>{copy.connections.endpoint}</span><Link href="/ai#tools">{copy.controls.explore}<span aria-hidden="true">↗</span></Link></div>
      </div>)}
    </div>
  </section>;
}

const publicCopySourcesForLocale={copy,clients};
