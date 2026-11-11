"use client";

import { useId } from 'react';
import Image from 'next/image';
import Link from '@/i18n/Link';
import { assetPaths } from '@/assets/asset-paths';
import { BrandLogo } from '@/components/brand/BrandLogo/BrandLogo';
import { LandingArrow, LandingLink, LandingSection, LandingToolIcon } from '@/components/public/landing/LandingPrimitives';
import type { HomeEnglishCopy } from '@/i18n/public/home-en';
import { HomePicture } from './HomeMedia';
import styles from './HomeWorkflow.module.css';

/** Illustrations are decorative; each complete card is one ordinary, working link. */
function CatalogueIllustration({ copy }: { copy: HomeEnglishCopy }) {
  const photos = ['suiteImage', 'suiteVideo', 'suiteAudio', 'suiteText'] as const;
  const tools = [copy.tools.items[0], copy.tools.items[1], copy.tools.items[2], copy.tools.items[4]];
  return <div className={styles.catalogue} aria-hidden="true" data-workflow-art="catalogue">
    <div className={styles.filters}>{copy.tools.filters.slice(0, 4).map((label, index) => <span key={label} data-selected={index === 0}>{label}</span>)}</div>
    <div className={styles.toolGrid}>{tools.map((tool, index) => <div className={styles.toolPreview} key={tool.title}>
      <HomePicture photo={photos[index]} sizes="(max-width: 767px) 45vw, 180px" />
      <span><LandingToolIcon name={tool.icon} /><b>{tool.title}</b></span>
    </div>)}</div>
  </div>;
}

function ProcessIllustration() {
  return <div className={styles.process} aria-hidden="true" data-workflow-art="process">
    <svg className={styles.connections} viewBox="0 0 360 310" fill="none" preserveAspectRatio="none">
      <path d="M-30 118H80C170 118 123 55 218 55H385" stroke="var(--landing-accent)" strokeWidth="5" />
      <path d="M30 205H112C190 205 143 265 255 265H385" stroke="var(--landing-muted-dark)" strokeWidth="5" />
    </svg>
    <div className={styles.conversation}><BrandLogo size={22} /><div><i /><i /><i /></div></div>
    <div className={styles.result} data-result="image"><HomePicture photo="suiteImage" sizes="180px" /><span><LandingToolIcon name="image" /></span></div>
    <div className={styles.result} data-result="video"><HomePicture photo="suiteVideo" sizes="160px" /><span><LandingToolIcon name="video" /></span></div>
  </div>;
}

function FilesIllustration() {
  return <div className={styles.files} aria-hidden="true" data-workflow-art="files">
    <div className={styles.backSheet} />
    <div className={styles.fileWindow}><Image src={assetPaths.images.home.ui.files} width={976} height={528} sizes="(max-width: 767px) 85vw, 410px" alt="" data-workflow-ui="files" /></div>
  </div>;
}

function StartIllustration({ copy }: { copy: HomeEnglishCopy }) {
  return <div className={styles.start} aria-hidden="true" data-workflow-art="start">
    {(['suiteInspiration', 'suiteImage', 'suiteVideo'] as const).map((photo, index) => <div className={styles.startPreview} data-preview={index} key={photo}><HomePicture photo={photo} sizes="180px" /></div>)}
    <span className={styles.startAction}><BrandLogo size={18} />{copy.start}<LandingArrow /></span>
    <svg className={styles.pointer} width="36" height="42" viewBox="0 0 36 42" fill="none"><path d="m4 3 26 21-12 2-6 11Z" fill="white" stroke="var(--landing-ink)" strokeWidth="2" strokeLinejoin="round" /></svg>
  </div>;
}

export function HomeWorkflow({ copy }: { copy: HomeEnglishCopy }) {
  const id = useId();
  const illustrations = [<CatalogueIllustration copy={copy} key="catalogue" />, <ProcessIllustration key="process" />, <FilesIllustration key="files" />, <StartIllustration copy={copy} key="start" />];
  return <LandingSection id="workflow" title={copy.workflow.title} description={copy.workflow.description} align="start" className={styles.section} action={<LandingLink href="/app">{copy.start}<LandingArrow /></LandingLink>}>
    <div className={styles.grid} data-workflow-grid>{copy.workflow.cards.map((card, index) => <Link href={card.href} key={card.title} className={styles.card} data-card={index} aria-labelledby={`${id}-workflow-${index}`}>
      <div className={styles.copy} data-workflow-copy><h3 id={`${id}-workflow-${index}`}>{card.title}</h3><p>{card.description}</p></div>
      {illustrations[index]}
      <span className={styles.arrow} aria-hidden="true"><LandingArrow /></span>
    </Link>)}</div>
  </LandingSection>;
}
