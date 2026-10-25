"use client";
import {avatarModelsEn} from "@/i18n/public/avatar-models-en";
import {comparisonEn as copy} from "@/i18n/public/comparison-en";
import Link from "@/i18n/Link";
import styles from "@/components/public/editorial/Editorial.module.css";
import {usePublicCopy} from "@/i18n/public/use-public-copy";



function ReferenceSummary({model}:{model:typeof avatarModelsEn[number]}) {
  const {copy} = usePublicCopy(publicCopySourcesForLocale);

  const refs=model.references,refText=refs.support==='supported'?`${copy.fields.max} ${refs.maxCount} ${copy.fields.images} (${refs.extensions.join(', ').toUpperCase()})`:refs.support==='unsupported'?copy.fields.notExposed:copy.fields.unspecified;
  return <p className={styles.modelFacts}><strong>{copy.fields.references}:</strong> {refText}. <strong>{copy.fields.resolutions}:</strong> {model.resolutions?.join(', ')||copy.fields.unspecified}. <strong>{copy.fields.ratios}:</strong> {model.ratios?.join(', ')||copy.fields.unspecified}. <strong>{copy.fields.outputs}:</strong> {model.maxOutputs??copy.fields.unspecified}. {model.qualityModes?.length?<><strong>{copy.fields.quality}:</strong> {model.qualityModes.join(', ')}. </>:null}<strong>{copy.fields.combined}:</strong> {model.maxCombinedImages??copy.fields.unspecified}.</p>;
}

export const comparisonReadingMinutes=Math.max(1,Math.ceil([copy.intro,copy.methodology,...copy.sections.flatMap(s=>[s.title,...s.paragraphs,...(s.bullets??[])]),...Object.values(copy.models).flatMap(m=>[m.description,m.review]),...copy.faq.flatMap(f=>[f.question,f.answer])].join(' ').split(/\s+/).length/200));

export function ComparisonArticle() {
  const {copy,avatarModelsEn} = usePublicCopy(publicCopySourcesForLocale);

  return <article className={styles.article} aria-label={copy.title}><p>{copy.shortlist}</p><ol className={styles.shortlist}>{avatarModelsEn.map(m=><li key={m.id}><a href={`#model-${m.slug}`}>{m.name}</a></li>)}</ol><p id="methodology" className={styles.methodology}>{copy.methodology}</p><nav aria-label={copy.outline} className={styles.inlineOutline}><p>{copy.outline}</p><ol>{copy.sections.map(s=><li key={s.id}><a href={`#${s.id}`}>{s.label}</a>{s.kind==='models'?<ul>{avatarModelsEn.map(m=><li key={m.id}><a href={`#model-${m.slug}`}>{m.name}</a></li>)}</ul>:null}</li>)}</ol></nav>
    {copy.sections.map(section=><section key={section.id} id={section.id} className={styles.chapter} data-comparison-chapter><h2>{section.title}</h2>{section.paragraphs.map(p=><p key={p}>{p}</p>)}{section.bullets?<ul>{section.bullets.map(p=><li key={p}>{p}</li>)}</ul>:null}
      {section.kind==='models'?<><p className={styles.snapshot}>{copy.snapshot}</p>{avatarModelsEn.map((m,index)=><div key={m.id} id={`model-${m.slug}`} className={styles.model} data-comparison-model><h3>{index+1}. {m.name}</h3><p>{copy.models[m.id].description}</p><ReferenceSummary model={m}/><p>{copy.models[m.id].review} <Link href={`/app?model=${encodeURIComponent(m.id)}`}>{copy.fields.review}</Link></p></div>)}<p>{copy.scope}</p></>:null}
      {section.id==='workflow'?<p><Link href={copy.cta.href}>{copy.cta.action}</Link></p>:null}
      {section.id==='responsible-use'?<p><Link href="/legal/user-agreement">{copy.terms}</Link> · <Link href="/legal/privacy-policy">{copy.privacy}</Link></p>:null}
      {section.kind==='faq'?<dl className={styles.faq}>{copy.faq.map(f=><div key={f.question} data-comparison-faq><dt>{f.question}</dt><dd>{f.answer}</dd></div>)}</dl>:null}
    </section>)}
    <div className={styles.cta}><span>{copy.cta.eyebrow}</span><h2>{copy.cta.title}</h2><p>{copy.cta.description}</p><Link href={copy.cta.href}>{copy.cta.action}<span aria-hidden="true">↗</span></Link></div>
  </article>;
}



const publicCopySourcesForLocale={avatarModelsEn,copy,comparisonReadingMinutes};
