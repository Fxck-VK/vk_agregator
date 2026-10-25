"use client";
import {BrandLogo} from "@/components/brand/BrandLogo/BrandLogo";
import {BrandName} from "@/components/brand/BrandName/BrandName";
import {LandingToolIcon} from "@/components/public/landing/LandingPrimitives";
import {MediaImage} from "@/components/media/MediaImage/MediaImage";
import {pricingEn as t} from "@/i18n/public/pricing-en";
import {pricingReference as reference} from "@/i18n/public/pricing-reference-en";
import Link from "@/i18n/Link";
import styles from "./Pricing.module.css";
import {usePublicCopy} from "@/i18n/public/use-public-copy";

export function PricingBanner(){
  const {reference,t} = usePublicCopy(publicCopySourcesForLocale);
const image=reference.background;return <section className={styles.banner}><div className={styles.bannerBackground}><MediaImage src={image.src} alt="" width={image.width} height={image.height} fit="cover" passive loading="lazy" showLoading={false}/></div><div><span className={styles.eyebrow}>{t.banner.eyebrow}</span><h2>{t.banner.title}</h2><p>{t.banner.description}</p><Link href="/app" className={styles.lightButton}>{t.banner.action}<span aria-hidden="true">↗</span></Link></div><div className={styles.brandCanvas} aria-hidden="true"><div className={styles.canvasBrand}><BrandLogo size={42}/><BrandName/></div><div className={styles.canvasTiles}><span>✧</span><span>◈</span><span>♪</span></div><div className={styles.canvasLine}/><div className={styles.canvasLine}/></div></section>;}
export function PricingTools(){
  const {t} = usePublicCopy(publicCopySourcesForLocale);
return <section className={styles.tools}><h2>{t.toolTitle}</h2><p>{t.toolDescription}</p><div>{t.tools.map(tool=><Link href={tool.href} key={tool.name}><LandingToolIcon name={tool.icon}/><span>{tool.name}</span></Link>)}</div></section>;}
export function PricingStories(){
  const {t} = usePublicCopy(publicCopySourcesForLocale);
return <section className={styles.stories}><h2>{t.storiesTitle}</h2><p>{t.storiesDescription}</p><div>{t.stories.map((story,index)=><Link href={story.href} key={story.title}><span className={styles.storyArt} data-index={index} aria-hidden="true"><BrandLogo size={48}/><span>{['✧','◈','↗'][index]}</span></span><h3>{story.title}</h3><p>{story.description}</p><span aria-hidden="true">↗</span></Link>)}</div></section>;}

const publicCopySourcesForLocale={t,reference};
