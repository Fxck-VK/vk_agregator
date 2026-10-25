"use client";
import { LandingArrow } from "@/components/public/landing/LandingPrimitives";
import { klingEn as copy } from "@/i18n/public/kling-en";
import { klingReferenceEn as reference } from "@/i18n/public/kling-reference-en";
import Link from "@/i18n/Link";
import { FamilyPicture } from "./FamilyMedia";
import { FamilyChapter, modelHref } from "./FamilySections";
import layout from "@/components/public/generator-landing/GeneratorLanding.module.css";
import styles from "@/components/public/generator-landing/ModelLanding.module.css";
import {usePublicCopy} from "@/i18n/public/use-public-copy";


export function FamilyPlans() {
  const {copy} = usePublicCopy(publicCopySourcesForLocale);

  return <FamilyChapter id="plans" {...copy.plans}><div className={layout.plansTiers}>{copy.plans.cards.map((card,index)=><article className={`${layout.planTier} ${index===1?layout.planTierFeatured:""}`} key={card.name}><h3 className={layout.planTierName}>{card.name}</h3><p className={layout.planTierPrice}>{card.price}</p><p className={layout.planTierNote}>{card.note}</p><p className={layout.planTierLine}>{card.body}</p><Link href={card.href} className={`${layout.planTierCta} ${index===1?layout.planTierCtaSolid:""}`}>{card.action}</Link></article>)}</div><p className={layout.plansNote}>{copy.plans.note}</p><Link href="/legal/paid-access" className={styles.businessLink}>{copy.plans.terms} →</Link></FamilyChapter>;
}
export function FamilyFinale() {
  const {copy,reference} = usePublicCopy(publicCopySourcesForLocale);

  return <><section id="questions" className={layout.faq} aria-labelledby="family-questions-title"><h2 id="family-questions-title" className={layout.faqTitle}>{copy.faqTitle}</h2><div className={layout.faqList}>{copy.questions.map(item=><details key={item.q} className={layout.faqItem}><summary className={layout.faqSummary}><span>{item.q}</span><span className={layout.faqPlus} aria-hidden="true">+</span></summary><p className={layout.faqAnswer}>{item.a}</p></details>)}</div></section><section className={layout.closingBanner} aria-labelledby="family-closing-title"><FamilyPicture image={reference.closing} className={layout.closingBannerBg} sizes="100vw" decorative/><div className={layout.closingBannerOverlay}/><div className={layout.closingBannerInner}><h2 id="family-closing-title" className={layout.closingBannerTitle}>{copy.closing.title}</h2><p className={layout.closingBannerBody}>{copy.closing.body}</p><div className={layout.closingBannerCtas}><Link href={modelHref("video_kling_v3")} className={layout.closingBannerCta}>{copy.closing.action}</Link><a href="#versions" className={layout.closingBannerCtaSecondary}>{copy.closing.secondary}</a></div></div></section><section className={layout.related} aria-labelledby="family-related-title"><div className={layout.relatedHeader}><h2 id="family-related-title" className={layout.relatedTitle}>{copy.related.title}</h2><Link href="/ai" className={layout.relatedSeeAll}>{copy.related.all}<LandingArrow/></Link></div><div className={layout.relatedRows}>{copy.related.groups.map(group=><div className={layout.relatedRow} key={group.title}><h3 className={layout.relatedGroup}>{group.title}</h3><ul className={layout.relatedList}>{group.links.map(link=><li key={link.label}><Link href={link.href} className={layout.relatedLink}>{link.label}<span className={layout.relatedArrow}><LandingArrow/></span></Link></li>)}</ul></div>)}</div></section></>;
}

const publicCopySourcesForLocale={copy,reference};
