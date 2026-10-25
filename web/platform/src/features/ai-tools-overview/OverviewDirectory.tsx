"use client";

import { useState } from "react";
import { aiToolsEn as copy, type OverviewAction, type OverviewCategory } from "@/i18n/public/ai-tools-en";
import { aiToolsReferenceEn as reference } from "@/i18n/public/ai-tools-reference-en";
import { OverviewActionLink, OverviewHeading, OverviewIcon, OverviewTabs } from "./OverviewPrimitives";
import styles from "./ToolsOverview.module.css";
import {usePublicCopy} from "@/i18n/public/use-public-copy";


const categories: readonly OverviewCategory[] = reference.categories;

export function OverviewDirectory({ onPreview }: { onPreview: (item: OverviewAction) => void }) {
  const {categories,copy} = usePublicCopy(publicCopySourcesForLocale);

  const [selected, setSelected] = useState(categories[0].id);
  return <section id="tools" className={styles.directory}>
    <OverviewHeading {...copy.directory} />
    <OverviewTabs prefix="tools" label={copy.controls.tools} items={categories} selected={selected} onChange={setSelected} className={styles.toolTabs} />
    {categories.map(category => <div key={category.id} className={styles.toolGrid} role="tabpanel" id={`tools-panel-${category.id}`} aria-labelledby={`tools-tab-${category.id}`} hidden={selected !== category.id} data-tone={category.tone} tabIndex={0}>
      {category.tools.map(tool => <OverviewActionLink item={tool} onPreview={onPreview} className={styles.toolItem} key={tool.title}>
        <span className={styles.toolIcon}><OverviewIcon name={tool.icon} /></span><span className={styles.toolText}><span className={styles.toolTitle}>{tool.title}{tool.planned ? <small>{copy.controls.comingSoon}</small> : null}</span><span className={styles.toolDescription}>{tool.description}</span></span>
      </OverviewActionLink>)}
    </div>)}
  </section>;
}

const publicCopySourcesForLocale={copy,reference,categories};
