"use client";
import { GeneratorChapter } from "@/components/public/generator-landing/GeneratorChapter";
import { ModelPicture } from "@/components/public/generator-landing/ModelMedia";
import { LandingArrow } from "@/components/public/landing/LandingPrimitives";
import { kling30En as copy } from "@/i18n/public/kling30-en";
import { kling30ReferenceEn as reference } from "@/i18n/public/kling30-reference-en";
import Link from "@/i18n/Link";
import layout from "@/components/public/generator-landing/GeneratorLanding.module.css";
import model from "@/components/public/generator-landing/ModelLanding.module.css";
import styles from "./ModelVersion.module.css";
import {usePublicCopy} from "@/i18n/public/use-public-copy";
import {translatePublicText} from "@/i18n/public/public-copy";
import {useLocale} from "@/i18n/LocaleProvider";



export function Kling30Details(){
  const publicCopyLocale=useLocale();

  const {copy} = usePublicCopy(publicCopySourcesForLocale);

  return <><GeneratorChapter id="whats-new" eyebrow={copy.whatsNew.eyebrow} title={copy.whatsNew.title}><p className={styles.whatsNewBody}>{copy.whatsNew.body}</p><Link href="/ai/kling#whats-new" className={styles.whatsNewLink}>{copy.whatsNew.action} →</Link></GeneratorChapter><GeneratorChapter id="specs" {...copy.specs}><div className={layout.modelsTableCard}><div className={layout.modelsTableScroll} role="region" aria-label={translatePublicText("Kling 3.0 specifications",publicCopyLocale)} tabIndex={0}><table className={`${layout.modelsTable} ${model.specTable} ${styles.specsTable}`}><caption className={layout.srOnly}>{copy.specs.title}</caption><tbody>{copy.specs.rows.map(row=><tr key={row.label}><th scope="row">{row.label}</th><td><span>{translatePublicText(row.value,publicCopyLocale)}</span>{"note"in row?<span className={model.cellNote}>{row.note}</span>:null}</td></tr>)}</tbody></table></div></div><div className={styles.specsFoot}><p className={model.tableNote}>{copy.specs.note}</p><Link href="/ai/kling#versions" className={styles.inlineLink}>{copy.specs.compare} →</Link></div></GeneratorChapter></>;
}
export function Kling30Scenarios(){
  const {copy,reference} = usePublicCopy(publicCopySourcesForLocale);

  return <GeneratorChapter id="best-for" {...copy.best}><div className={`${layout.useCaseGrid} ${styles.bestForGrid}`}>{copy.best.cards.map((card,index)=><article key={card.title} className={layout.useCaseCard}><div className={layout.useCaseImgWrap}><ModelPicture image={reference.best[index]} assetBase="model-version" className={layout.useCaseImg} sizes="(max-width: 820px) 90vw, 25vw"/></div><div className={layout.useCaseBody}><h3 className={layout.useCaseH3}>{card.title}</h3><p className={layout.useCaseLine}>{card.body}</p><Link href={card.href} className={model.chooseCta}>{card.action}<LandingArrow/></Link></div></article>)}</div><aside className={styles.stillChoose}><h3 className={styles.stillChooseTitle}>{copy.best.aside.title}</h3><p className={styles.stillChooseBody}>{copy.best.aside.body}</p><Link href="/ai/kling#versions" className={styles.stillChooseLink}>{copy.best.aside.action}</Link></aside></GeneratorChapter>;
}
export function Kling30Research(){
  const {copy,reference} = usePublicCopy(publicCopySourcesForLocale);

  return <GeneratorChapter id="numbers" eyebrow={copy.research.eyebrow} title={copy.research.title}><div className={`${layout.numbersBlock} ${model.research}`}><p className={model.researchLead}>{copy.research.intro}</p>{reference.stats.map(slot=><div className={layout.studySlot} key={slot.number} data-research-figure><p className={layout.studySlotNumber}>{slot.number}</p><div><p className={layout.studyBlockFigure}>{slot.figure}</p><h3 className={layout.studySlotHeading}>{slot.title}</h3><p className={layout.studyClaim}>{slot.claim}</p></div></div>)}<p className={model.numbersSource}>{copy.research.source} <a href="https://www.magnific.com/ai/kling/kling-3-0#numbers" target="_blank" rel="noreferrer">{copy.research.link}</a></p></div></GeneratorChapter>;
}
export function Kling30Finale(){
  const {copy,reference} = usePublicCopy(publicCopySourcesForLocale);

  return <><section id="questions" className={layout.faq} aria-labelledby="version-questions-title"><h2 id="version-questions-title" className={layout.faqTitle}>{copy.faqTitle}</h2><div className={layout.faqList}>{copy.questions.map(item=><details key={item.q} className={layout.faqItem}><summary className={layout.faqSummary}><span>{item.q}</span><span className={layout.faqPlus} aria-hidden="true">+</span></summary><p className={layout.faqAnswer}>{item.a}</p></details>)}</div></section><section className={layout.closingBanner} aria-labelledby="version-closing-title"><ModelPicture image={reference.closing} assetBase="model-version" className={layout.closingBannerBg} sizes="100vw" decorative/><div className={layout.closingBannerOverlay}/><div className={layout.closingBannerInner}><h2 id="version-closing-title" className={layout.closingBannerTitle}>{copy.closing.title}</h2><p className={layout.closingBannerBody}>{copy.closing.body}</p><div className={layout.closingBannerCtas}><Link href={`/app?model=${copy.modelId}`} className={layout.closingBannerCta}>{copy.closing.action}</Link><Link href="/ai/kling#versions" className={layout.closingBannerCtaSecondary}>{copy.closing.secondary}</Link></div></div></section><section className={layout.related} aria-labelledby="version-related-title"><div className={layout.relatedHeader}><h2 id="version-related-title" className={layout.relatedTitle}>{copy.related.title}</h2><Link href="/ai" className={layout.relatedSeeAll}>{copy.related.all}<LandingArrow/></Link></div><div className={layout.relatedRows}>{copy.related.groups.map(group=><div className={layout.relatedRow} key={group.title}><h3 className={layout.relatedGroup}>{group.title}</h3><ul className={layout.relatedList}>{group.links.map(link=><li key={link.label}><Link href={link.href} className={layout.relatedLink}>{link.label}<span className={layout.relatedArrow}><LandingArrow/></span></Link></li>)}</ul></div>)}</div></section></>;
}

const publicCopySourcesForLocale={copy,reference};
