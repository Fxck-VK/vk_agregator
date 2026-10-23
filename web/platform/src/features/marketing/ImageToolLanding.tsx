"use client";

import Image from "next/image";
import { useState } from "react";
import { PageContainer } from "@/components/public/PageContainer/PageContainer";
import { PrimaryButton } from "@/components/public/PrimaryButton/PrimaryButton";
import { FAQ } from "@/components/ui/FAQ/FAQ";
import { ModelIcon } from "@/features/models/ModelIcon/ModelIcon";
import { modelIconRegistry } from "@/features/models/model-icon-registry";
import { useDictionary } from "@/i18n/LocaleProvider";
import Link from "@/i18n/Link";
import styles from "./ImageToolLanding.module.css";

const pictures = ["product-shots.webp", "advertising.webp", "gpt-2.webp"];
const modelIds = ["gpt_image_2", "flux_2_pro", "seedream_5_0_lite"];
const sections = ["examples", "models", "how-it-works", "questions"];
const mediaBase = "/assets/images/marketing/";

export function ImageToolLanding() {
  const { imageLanding: t } = useDictionary();
  const [selected, setSelected] = useState(0);
  const current = t.examples[selected];

  return <div className={styles.page}>
    <PageContainer size="wide">
      <nav className={styles.breadcrumb} aria-label={t.breadcrumb}>
        <Link href="/">{t.home}</Link><span aria-hidden="true">/</span><span>{t.breadcrumb}</span>
      </nav>
      <section className={styles.hero} aria-labelledby="image-tool-title">
        <div className={styles.heroCopy}>
          <span className={styles.eyebrow}><span className={styles.spark} aria-hidden="true">✦</span>{t.eyebrow}</span>
          <h1 id="image-tool-title">{t.title}<span>{t.titleAccent}</span></h1>
          <p className={styles.intro}>{t.intro}</p>
          <div className={styles.actions}>
            <PrimaryButton href="/app/image" className={styles.primary}>{t.open}<span aria-hidden="true">{t.arrow}</span></PrimaryButton>
            <a className={styles.textLink} href="#examples">{t.explore}<span aria-hidden="true">↓</span></a>
          </div>
          <p className={styles.reassurance}>{t.reassurance}</p>
          <div className={styles.exampleButtons} role="group" aria-label={t.exampleControls}>
            {t.examples.map((example, index) => <button key={pictures[index]} type="button" aria-pressed={selected === index} onClick={() => setSelected(index)}>{example.label}</button>)}
          </div>
        </div>
        <div className={styles.visual}>
          <div className={styles.heroPicture}>
            <Image key={pictures[selected]} src={mediaBase + pictures[selected]} alt={current.alt} fill priority sizes="(max-width: 800px) 90vw, 45vw" />
            <div className={styles.visualTag}><span aria-hidden="true">✦</span>{current.label}</div>
          </div>
          <div className={styles.promptCard} aria-live="polite">
            <span className={styles.promptLabel}>{t.promptLabel}</span>
            <p>{current.prompt}</p>
            <span className={styles.promptDecoration} aria-hidden="true">↗</span>
          </div>
        </div>
      </section>
      <nav className={styles.sectionNav} aria-label={t.navLabel}>
        {sections.map((id, index) => <a key={id} href={`#${id}`}><span aria-hidden="true">0{index + 1}</span>{t.nav[index]}</a>)}
      </nav>

      <section id="examples" className={styles.section} aria-labelledby="image-examples-title">
        <div className={styles.sectionHeading}><span className={styles.eyebrow}>{t.galleryEyebrow}</span><h2 id="image-examples-title">{t.galleryTitle}</h2><p>{t.galleryIntro}</p></div>
        <div className={styles.gallery}>
          {t.examples.map((example, index) => <figure key={pictures[index]}>
            <div className={styles.galleryPicture}><Image src={mediaBase + pictures[index]} alt={example.alt} fill loading="lazy" sizes="(max-width: 650px) 90vw, 30vw" /><span>{example.label}</span></div>
            <figcaption><h3>{example.title}</h3><p>{example.prompt}</p></figcaption>
          </figure>)}
        </div>
        <p className={styles.mediaNote}>{t.exampleNote}</p>
      </section>

      <section id="models" className={`${styles.section} ${styles.modelsSection}`} aria-labelledby="image-models-title">
        <div className={styles.splitHeading}><div><span className={styles.eyebrow}>{t.modelsEyebrow}</span><h2 id="image-models-title">{t.modelsTitle}</h2></div><p>{t.modelsIntro}</p></div>
        <div className={styles.models}>
          {modelIds.map((id, index) => <Link key={id} className={styles.model} href="/app/models" aria-label={`${t.modelLink}: ${t.modelNames[index]}`}><ModelIcon src={modelIconRegistry[id]} className={styles.modelIcon} /><div><h3>{t.modelNames[index]}</h3><span>{t.modelDevelopers[index]}</span></div><span className={styles.modelArrow} aria-hidden="true">↗</span></Link>)}
        </div>
      </section>

      <section id="how-it-works" className={styles.section} aria-labelledby="image-steps-title">
        <div className={styles.sectionHeading}><span className={styles.eyebrow}>{t.howEyebrow}</span><h2 id="image-steps-title">{t.howTitle}</h2></div>
        <ol className={styles.steps}>{t.steps.map((step, index) => <li key={step.title}><span className={styles.stepNumber}>0{index + 1}</span><h3>{step.title}</h3><p>{step.body}</p></li>)}</ol>
      </section>

      <section id="questions" className={`${styles.section} ${styles.faqSection}`} aria-labelledby="image-faq-title">
        <div><span className={styles.eyebrow}>{t.faqEyebrow}</span><h2 id="image-faq-title">{t.faqTitle}</h2></div>
        <FAQ items={t.faq} name="image-tool-preview-faq" />
      </section>

      <section className={styles.final} aria-labelledby="image-final-title"><span className={styles.finalSpark} aria-hidden="true">✦</span><h2 id="image-final-title">{t.finalTitle}</h2><p>{t.finalBody}</p><PrimaryButton href="/app/image" className={styles.primary}>{t.open}<span aria-hidden="true">↗</span></PrimaryButton></section>
      <p className={styles.previewLabel}>{t.previewLabel}</p>
    </PageContainer>
  </div>;
}
