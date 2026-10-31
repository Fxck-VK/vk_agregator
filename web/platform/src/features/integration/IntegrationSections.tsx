"use client";
import {LandingToolIcon} from "@/components/public/landing/LandingPrimitives";
import {MediaImage} from "@/components/media/MediaImage/MediaImage";
import {integrationReference as reference} from "@/i18n/public/integration-reference-en";
import {mcpEn as t} from "@/i18n/public/integration-en";
import Link from "@/i18n/Link";
import styles from "./Integration.module.css";
import {usePublicCopy} from "@/i18n/public/use-public-copy";
import {translatePublicText,translatePublicMessage} from "@/i18n/public/public-copy";
import {useLocale} from "@/i18n/LocaleProvider";


export function IntegrationCapabilities({onVideo}:{onVideo:()=>void}){
  const publicCopyLocale=useLocale();

  const {reference,t} = usePublicCopy(publicCopySourcesForLocale,{"reference":"integrationReference","t":"mcpEn"});

 const image=reference.images.overview;
 return <section className={styles.capabilities} aria-labelledby="integration-capabilities-title"><h2 id="integration-capabilities-title">{t.capability.title}</h2><p>{t.capability.description}</p><ul className={styles.capabilityGrid}>{t.capability.items.map(item=><li key={item.title}><span className={styles.toolIcon}><LandingToolIcon name={item.icon}/></span><div><h3><Link href={item.href}>{item.title}</Link></h3><p>{item.description}</p></div></li>)}</ul><figure className={styles.videoReference}><button type="button" onClick={onVideo} aria-label={t.controls.video}><MediaImage src={image.src} alt={translatePublicText("Magnific MCP source demonstration preview",publicCopyLocale)} width={image.width} height={image.height} fit="cover" loading="lazy" passive showLoading={false}/><span className={styles.play}>▶ <span>{t.controls.video}</span></span></button><figcaption>{t.controls.reference}</figcaption></figure></section>;
}
export function IntegrationFeatures({onConnect}:{onConnect:()=>void}){
  const publicCopyLocale=useLocale();

  const {t} = usePublicCopy(publicCopySourcesForLocale,{"reference":"integrationReference","t":"mcpEn"});

 return <section className={styles.features} aria-label={translatePublicText("Integration directions",publicCopyLocale)}>{t.features.map((feature,index)=><article key={feature.title} data-integration-feature className={styles.feature} data-reverse={index%2===1||undefined}><div><h2>{feature.title}</h2><p>{feature.description}</p><button type="button" className={styles.outline} onClick={onConnect}>{t.controls.connect}</button></div><figure><MediaImage src={feature.image.src} alt={translatePublicMessage('Magnific reference: {title}',publicCopyLocale,{title:feature.title})} width={feature.image.width} height={feature.image.height} fit="cover" passive loading="lazy" showLoading={false}/><figcaption>{t.controls.reference}</figcaption></figure></article>)}</section>;
}
export function IntegrationComparison(){
  const {t} = usePublicCopy(publicCopySourcesForLocale,{"reference":"integrationReference","t":"mcpEn"});

 return <section className={styles.comparison} aria-labelledby="integration-comparison-title"><h2 id="integration-comparison-title">{t.comparison.title}</h2><p>{t.comparison.description}</p><div className={styles.comparisonGrid}>{t.comparison.cards.map((card,index)=><article key={card.title}><span className={styles.comparisonIcon} aria-hidden="true">{index===0?"⌘":"✧"}</span><h3>{card.title}</h3><strong>{card.subtitle}</strong><p>{card.description}</p><ul>{card.points.map(point=><li key={point}><span aria-hidden="true">→</span>{point}</li>)}</ul><small>{card.status}</small></article>)}</div><p className={styles.comparisonNote}>{t.comparison.note} <Link href="/workflows/product-campaign">{t.comparison.link} ↗</Link></p></section>;
}

const publicCopySourcesForLocale={reference,t};
