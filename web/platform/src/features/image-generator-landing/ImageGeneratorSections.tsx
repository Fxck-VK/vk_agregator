"use client";

import type { ReactNode } from "react";
import { LandingArrow, LandingToolIcon } from "@/components/public/landing/LandingPrimitives";
import { imageGeneratorEn as copy } from "@/i18n/public/image-generator-en";
import { imageGeneratorReferenceEn as reference } from "@/i18n/public/image-generator-reference-en";
import Link from "@/i18n/Link";
import { GeneratorPicture, type ReferenceImage } from "./ImageGeneratorMedia";
import styles from "@/components/public/generator-landing/GeneratorLanding.module.css";
import {usePublicCopy} from "@/i18n/public/use-public-copy";


function Chapter({ id, eyebrow, title, intro, children }: { id: string; eyebrow: string; title: string; intro?: string; children: ReactNode }) {
  return <section id={id} className={styles.chapter} aria-labelledby={`${id}-title`}><p className={styles.eyebrow}>{eyebrow}</p><h2 id={`${id}-title`} className={styles.chapterTitle}>{title}</h2>{intro ? <p className={styles.chapterIntro}>{intro}</p> : null}{children}</section>;
}
function TextLink({ href, children }: { href: string; children: ReactNode }) { return <Link href={href} className={styles.useCaseLink}><span>{children}</span><span className={styles.useCaseArrow}><LandingArrow /></span></Link>; }

export function ImageGeneratorModels() {
  const {copy,reference} = usePublicCopy(publicCopySourcesForLocale,{"copy":"imageGeneratorEn","reference":"imageGeneratorReferenceEn"});

  return <Chapter id="models" {...copy.models}>
    <div className={styles.modelsTableCard}><div className={styles.modelsTableScroll} role="region" aria-label={copy.models.title} tabIndex={0}><table className={styles.modelsTable}><thead><tr>{copy.models.columns.map(label => <th scope="col" key={label}>{label}</th>)}</tr></thead><tbody>{copy.models.rows.map(row => <tr key={row.need}><th scope="row" className={styles.modelsWhen}>{row.need}</th><td><div className={styles.modelChips}>{row.models.map(name => <Link href="/app/models" className={styles.modelChip} key={name}>{name}</Link>)}</div></td><td className={styles.modelsWhy}>{row.why}</td><td className={styles.modelsOutput}><Link href="/app/models" className={styles.modelsOutputBadge}>{row.output}</Link></td></tr>)}</tbody></table></div></div>
    <div className={styles.autoBanner}><div className={styles.autoBannerText}><h3 className={styles.autoTitle}>{copy.models.autoTitle}</h3><p className={styles.autoBody}>{copy.models.autoBody}</p></div><Link href="/app/models" className={styles.autoCta}>{copy.models.autoAction}</Link></div>
    <div className={styles.familyHead}><h3 className={styles.familyTitle}>{copy.models.familiesTitle}</h3><p className={styles.familyMeta}>{copy.models.familiesMeta}</p></div>
    <div className={styles.familyMarquee}>{[0,1].map(track => <div className={styles.familyTrack} key={track} aria-hidden={track === 1 || undefined}>{reference.families.map(family => <Link href="/app/models" tabIndex={track ? -1 : 0} className={styles.familyCard} key={family.name}><GeneratorPicture image={family.image} className={styles.familyCardImg} sizes="200px" decorative={track === 1} /><span className={styles.familyCardMeta}><span className={styles.familyCardName}>{family.name}</span><span className={styles.familyCardProvider}>{family.caption} · {copy.models.familyAction}</span></span></Link>)}</div>)}</div>
  </Chapter>;
}

export function ImageGeneratorUseCases({ onOpen }: { onOpen: (image: ReferenceImage) => void }) {
  const {copy,reference} = usePublicCopy(publicCopySourcesForLocale,{"copy":"imageGeneratorEn","reference":"imageGeneratorReferenceEn"});

  return <Chapter id="use-cases" {...copy.useCases}><div className={styles.useCaseGrid}>{copy.useCases.cards.map((card, index) => <article className={styles.useCaseCard} key={card.label}>
    <div className={styles.useCaseImgWrap}><GeneratorPicture image={reference.useCaseImages[index]} className={styles.useCaseImg} /><div className={styles.useCaseChips}>{card.models.map(model => <Link href="/app/models" className={styles.useCaseChip} key={model}>{model}</Link>)}</div></div>
    <div className={styles.useCaseBody}><p className={styles.useCaseLabel}>{card.label}</p><h3 className={styles.useCaseH3}>{card.title}</h3><p className={styles.useCaseLine}>{card.body}</p><div className={styles.useCaseLinks}>{card.links.map(link => <TextLink href={link.href} key={link.label}>{link.label}</TextLink>)}</div></div>
  </article>)}</div><div className={styles.watchRow}><p className={styles.watchTitle}>{copy.useCases.tutorialsTitle}</p><div className={styles.watchGrid}>{reference.tutorials.map((item, index) => <button type="button" className={styles.watchItem} key={item.image.file} onClick={() => onOpen(item.image)}><span className={styles.watchThumb}><GeneratorPicture image={item.image} sizes="120px" /></span><span className={styles.watchLabel}>{copy.useCases.tutorialTitles[index]}</span><LandingArrow /></button>)}</div></div></Chapter>;
}

export function ImageGeneratorWorkflow() {
  const {copy,reference} = usePublicCopy(publicCopySourcesForLocale,{"copy":"imageGeneratorEn","reference":"imageGeneratorReferenceEn"});

  return <Chapter id="workflow" {...copy.workflow}><ul className={styles.workflowSteps}>{copy.workflow.steps.map(step => <li key={step.title}><Link href={step.href} className={styles.workflowStepInner}><span className={styles.workflowIcon}><LandingToolIcon name={step.icon} /></span><span className={styles.workflowStepLabel}>{step.title}</span><span className={styles.workflowStepLine}>{step.body}</span></Link></li>)}</ul><div className={styles.workflowCards}>{copy.workflow.cards.map((card, index) => <article className={styles.workflowCard} key={card.title}><div className={styles.workflowCardImg}><GeneratorPicture image={reference.workflowImages[index]} sizes="(max-width: 820px) 92vw, 26vw" /></div><div className={styles.workflowCardBodyWrap}><h3 className={styles.workflowCardTitle}>{card.title}</h3><p className={styles.workflowCardBody}>{card.body}</p><div className={styles.useCaseLinks}>{card.href ? <TextLink href={card.href}>{card.action}</TextLink> : <span className={styles.upcoming}>{card.action}</span>}</div></div></article>)}</div></Chapter>;
}

export function ImageGeneratorTrust() {
  const {copy} = usePublicCopy(publicCopySourcesForLocale,{"copy":"imageGeneratorEn","reference":"imageGeneratorReferenceEn"});

  return <section id="trust" className={styles.chapter} aria-labelledby="trust-title"><p className={styles.eyebrow}>{copy.trust.eyebrow}</p><div className={styles.trustHead}><div className={styles.trustHeadText}><h2 id="trust-title" className={styles.chapterTitle}>{copy.trust.title}</h2><p className={styles.chapterIntro}>{copy.trust.intro}</p></div><Link href="/legal" className={styles.trustRanking}><span className={styles.trustRankingBadge}>{copy.trust.badge}<span className={styles.trustRankingScale}>{copy.trust.badgeSmall}</span></span><span><span className={styles.trustRankingTitle}>{copy.trust.badgeTitle}</span><span className={styles.trustRankingSource}>{copy.trust.badgeLink}</span></span></Link></div><div className={styles.trustGrid}>{copy.trust.tiles.map((tile,index) => <div className={styles.trustTile} key={tile.title}><span className={styles.trustTileNum}>{`0${index+1}`}</span><h3 className={styles.trustTileTitle}>{tile.title}</h3><p className={styles.trustTileLine}>{tile.body}</p></div>)}</div></section>;
}

export function ImageGeneratorPlans() {
  const {copy} = usePublicCopy(publicCopySourcesForLocale,{"copy":"imageGeneratorEn","reference":"imageGeneratorReferenceEn"});

  return <Chapter id="plans" {...copy.plans}><div className={styles.plansLead}><p>{copy.plans.lead}</p></div><div className={styles.plansTiers}>{copy.plans.cards.map((card,index) => <article className={`${styles.planTier} ${index ? styles.planTierFeatured : ""}`} key={card.name}><h3 className={styles.planTierName}>{card.name}</h3><p className={styles.planTierPrice}>{card.price}</p><p className={styles.planTierNote}>{card.note}</p><p className={styles.planTierLine}>{card.body}</p><Link href={card.href} className={`${styles.planTierCta} ${index ? styles.planTierCtaSolid : ""}`}>{card.action}</Link></article>)}</div><p className={styles.plansNote}>{copy.plans.note}</p></Chapter>;
}

export function ImageGeneratorFinale() {
  const {copy,reference} = usePublicCopy(publicCopySourcesForLocale,{"copy":"imageGeneratorEn","reference":"imageGeneratorReferenceEn"});

  return <><section id="questions" className={styles.faq} aria-labelledby="questions-title"><h2 id="questions-title" className={styles.faqTitle}>{copy.faqTitle}</h2><div className={styles.faqList}>{copy.questions.map(item => <details key={item.q} className={styles.faqItem}><summary className={styles.faqSummary}><span>{item.q}</span><span className={styles.faqPlus} aria-hidden="true">+</span></summary><p className={styles.faqAnswer}>{item.a}</p></details>)}</div></section>
    <section className={styles.closingBanner} aria-labelledby="closing-title"><GeneratorPicture image={reference.closing} className={styles.closingBannerBg} sizes="100vw" decorative /><div className={styles.closingBannerOverlay} /><div className={styles.closingBannerInner}><h2 id="closing-title" className={styles.closingBannerTitle}>{copy.closing.title}</h2><div className={styles.closingBannerCtas}><Link href="/app/image" className={styles.closingBannerCta}>{copy.closing.action}</Link></div></div></section>
    <section className={styles.lastUpdated}><dl className={styles.lastUpdatedGrid}><div className={styles.lastUpdatedItem}><dt className={styles.lastUpdatedLabel}>{copy.updated.label}</dt><dd className={styles.lastUpdatedDate}><time dateTime="2026-10-24">{copy.updated.date}</time></dd></div><div className={styles.lastUpdatedItem}><dt className={styles.lastUpdatedLabel}>{copy.updated.changeLabel}</dt><dd className={styles.lastUpdatedChange}>{copy.updated.change}</dd></div></dl></section>
    <section className={styles.related} aria-labelledby="related-title"><div className={styles.relatedHeader}><h2 id="related-title" className={styles.relatedTitle}>{copy.related.title}</h2><Link href="/app/models" className={styles.relatedSeeAll}>{copy.related.all}</Link></div><div className={styles.relatedRows}>{copy.related.groups.map(group => <div className={styles.relatedRow} key={group.title}><h3 className={styles.relatedGroup}>{group.title}</h3><ul className={styles.relatedList}>{group.links.map(link => <li key={link.label}><Link href={link.href} className={styles.relatedLink}>{link.label}<span className={styles.relatedArrow}><LandingArrow /></span></Link></li>)}</ul></div>)}</div></section>
  </>;
}

const publicCopySourcesForLocale={copy,reference};
