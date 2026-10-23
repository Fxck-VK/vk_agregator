"use client";
import { LegalLinks } from "@/features/legal/LegalLinks";

import Image from "next/image";
import { useState, type KeyboardEvent } from "react";
import { BrandLogo } from "@/components/brand/BrandLogo/BrandLogo";
import { BrandName } from "@/components/brand/BrandName/BrandName";
import { PageContainer } from "@/components/public/PageContainer/PageContainer";
import { PrimaryButton } from "@/components/public/PrimaryButton/PrimaryButton";
import { FAQ } from "@/components/ui/FAQ/FAQ";
import { brandName } from "@/config/brand";
import { useDictionary } from "@/i18n/LocaleProvider";
import Link from "@/i18n/Link";
import { marketingAssets, marketingPartners, modelCards, originalImages, showcase, supportEmail, toolCards, useCaseImages, workflowImages } from "./marketing-content";
import { MarketingHeader } from "./MarketingHeader";
import { MarketingVideo } from "./MarketingVideo";
import { MarketingDialog } from "./MarketingDialog";
import { MarketingCarousel } from "./MarketingCarousel";
import { MediaVideo } from "@/components/media/MediaVideo/MediaVideo";
import { MarketingFooterGroups } from "./MarketingFooterGroups";
import styles from "./MarketingHome.module.css";

function Photo({ file, className, priority = false, alt = "", sizes = "(max-width: 700px) 90vw, 40vw" }: { file: string; className?: string; priority?: boolean; alt?: string; sizes?: string }) {
  return <Image src={`${marketingAssets}${file}`} alt={alt} fill sizes={sizes} priority={priority} className={className} />;
}

export function MarketingHome() {
  const { marketing: t } = useDictionary();
  const [selected, setSelected] = useState(0);
  const [suite, setSuite] = useState(0);
  const [filter, setFilter] = useState(0);
  const [paused, setPaused] = useState(false);
  const [original, setOriginal] = useState(0);
  const [introOpen, setIntroOpen] = useState(false);

  function selectFromKeyboard(event: KeyboardEvent<HTMLButtonElement>, index: number) {
    let next: number;
    if (event.key === "ArrowRight" || event.key === "ArrowDown") next = (index + 1) % showcase.length;
    else if (event.key === "ArrowLeft" || event.key === "ArrowUp") next = (index + showcase.length - 1) % showcase.length;
    else if (event.key === "Home") next = 0;
    else if (event.key === "End") next = showcase.length - 1;
    else return;
    event.preventDefault(); setSelected(next);
    document.getElementById(`showcase-tab-${next}`)?.focus();
  }

  function selectSuiteFromKeyboard(event: KeyboardEvent<HTMLButtonElement>, index: number) {
    let next: number;
    if (event.key === "ArrowRight") next = (index + 1) % t.suiteTabs.length;
    else if (event.key === "ArrowLeft") next = (index + t.suiteTabs.length - 1) % t.suiteTabs.length;
    else if (event.key === "Home") next = 0;
    else if (event.key === "End") next = t.suiteTabs.length - 1;
    else return;
    event.preventDefault(); setSuite(next); setSelected(Math.max(0, next - 1));
    document.getElementById(`suite-tab-${next}`)?.focus();
  }

  return (
    <div className={styles.page} data-animation-paused={paused || undefined}>
      <a className={styles.skipLink} href="#public-main">{t.skip}</a>
      <Link href="#creative-suite" className={styles.announcement}><span>{t.announcement}</span><span aria-hidden="true">→</span></Link>
      <MarketingHeader t={t} />
      <main id="public-main">
      <div className={styles.hero}>
        <Photo file="hero.webp" className={styles.heroImage} priority sizes="100vw" />
        <div className={styles.heroShade} />
        <PageContainer size="wide" className={styles.heroInner}>
          <div className={styles.heroCopy}>
            <Link href="/app/models" className={styles.eyebrow}>{t.eyebrow}<span aria-hidden="true">↗</span></Link>
            <h1 id="home-heading">{t.title}</h1><p>{t.description}</p>
            <div className={styles.heroButtons}><PrimaryButton href="/app" className={styles.whiteButton}>{t.create}</PrimaryButton><button type="button" onClick={() => setIntroOpen(true)} className={styles.outlineButton}><span aria-hidden="true">▶</span>{t.why}</button></div>
          </div>
          <div className={styles.capabilityWindow} aria-hidden="true"><div className={styles.capabilities}>{[...t.capabilities, ...t.capabilities].map((label, index) => <p key={`${label}-${index}`}><span>▶</span>{label}</p>)}</div></div>
        </PageContainer>
        <div className={styles.brandStrip}><p>{t.modelCaption}</p><div>{marketingPartners.map(name => <span key={name}>{name}</span>)}</div></div>
        <button className={styles.motionButton} type="button" onClick={() => setPaused(!paused)} aria-pressed={paused} aria-label={paused ? t.play : t.pause}>{paused ? "▶" : "Ⅱ"}</button>
      </div>

        <section className={styles.suiteSection} id="creative-suite" aria-labelledby="suite-heading">
          <div className={styles.sectionIntro}><h2 id="suite-heading">{t.suiteTitle}</h2><p>{t.suiteDescription}</p></div>
          <div className={styles.suitePills} role="tablist" aria-label={t.quickCategories}>{t.suiteTabs.map((label, index) => <button type="button" role="tab" key={label} id={`suite-tab-${index}`} aria-controls="suite-panel" aria-selected={suite === index} tabIndex={suite === index ? 0 : -1} onKeyDown={event => selectSuiteFromKeyboard(event, index)} onClick={() => { setSuite(index); setSelected(Math.max(0, index - 1)); }}>{label}</button>)}</div>
          <div className={styles.showcase} id="suite-panel" role="tabpanel" aria-labelledby={`suite-tab-${suite}`} tabIndex={0}>
            <div className={styles.showcaseInformation}><h3>{suite === 0 ? t.panelTitle : t.categoryTitles[selected]}</h3><p>{suite === 0 ? t.panelDescription : t.categoryDescriptions[selected]}</p><Link href={suite === 0 ? "/app/models" : showcase[selected].href} className={styles.textLink}>{t.overview} →</Link>
              {suite === 0 ? <div className={styles.categoryTabs} role="tablist" aria-label={t.categoryNavigation}>{showcase.map((item, index) => <button type="button" role="tab" aria-label={t.categoryTabs[index]} aria-selected={selected === index} tabIndex={selected === index ? 0 : -1} aria-controls="showcase-panel" id={`showcase-tab-${index}`} key={item.href} onClick={() => setSelected(index)} onKeyDown={event => selectFromKeyboard(event, index)}><span className={styles.categoryLabel}><span aria-hidden="true">{item.icon}</span>{t.categoryTabs[index]}</span>{selected === index ? <span className={styles.categoryDescription}>{t.categoryDescriptions[index]}</span> : null}<span className={styles.categoryProgress} /></button>)}</div> : null}
            </div>
            <div className={styles.showcasePanel} id="showcase-panel" role={suite === 0 ? "tabpanel" : undefined} aria-labelledby={suite === 0 ? `showcase-tab-${selected}` : undefined} tabIndex={suite === 0 ? 0 : undefined}>
              <div className={styles.showcaseImage} key={showcase[selected].image}>{selected < 2 ? <MarketingVideo paused={paused} kind={selected === 0 ? "image" : "video"} /> : <Photo file={showcase[selected].image} sizes="(max-width: 700px) 90vw, 65vw" />}<div className={styles.floatingPrompt}><span>{t.categoryTitles[selected]}</span><Link href={showcase[selected].href} aria-label={t.create}>↑</Link></div></div>
              <div className={styles.showcaseCaption}><div><h3>{t.categoryTitles[selected]}</h3><p>{t.categoryDescriptions[selected]}</p></div><Link href={showcase[selected].href} aria-label={t.create}>↗</Link></div>
            </div>
          </div>
        </section>

        <section className={styles.modelsSection} aria-labelledby="models-heading"><div className={styles.sectionIntro}><h2 id="models-heading">{t.modelsTitle}</h2><p>{t.modelsDescription}</p></div>
          <MarketingCarousel label={t.modelsTitle} className={styles.modelTrack} auto paused={paused}>{modelCards.map(model => <Link href="/app/models" key={model.name} className={styles.modelCard}><Photo file={model.image} sizes="(max-width: 700px) 55vw, 19vw" /><strong>{model.name}</strong><span aria-hidden="true">↗</span></Link>)}</MarketingCarousel>
        </section>

        <section className={styles.workflowSection} id="workflow" aria-labelledby="workflow-heading"><PageContainer size="wide"><div className={styles.sectionIntro}><h2 id="workflow-heading">{t.workflowTitle}</h2><p>{t.workflowDescription}</p><PrimaryButton href="/app" className={styles.darkButton}>{t.create} <span aria-hidden="true">↗</span></PrimaryButton></div>
          <div className={styles.workflowGrid}>{t.workflowCards.map((card, index) => <article className={styles.workflowCard} key={card.title}><div className={styles.workflowArt}><Photo file={workflowImages[index]} sizes="(max-width: 700px) 90vw, 50vw" /></div><div><h3>{card.title}</h3><p>{card.description}</p></div></article>)}</div>
        </PageContainer></section>

        <section className={styles.toolsSection} id="tools" aria-labelledby="tools-heading"><PageContainer size="wide"><div className={styles.splitHeading}><div><h2 id="tools-heading">{t.toolsTitle}</h2><p>{t.toolsDescription}</p></div><Link href="/app/models" className={styles.textLink}>{t.allTools} ↗</Link></div>
          <div className={styles.toolFilters} aria-label={t.toolsRegion}>{t.filters.map((label, index) => <button type="button" key={label} aria-pressed={filter === index} onClick={() => setFilter(index)}>{label}</button>)}</div>
          <MarketingCarousel key={filter} label={t.toolsRegion} className={styles.toolGrid}>{toolCards.map((card, index) => filter === 0 || filter === card.category ? <Link href={card.href} className={styles.toolCard} key={card.href + index}><div className={styles.toolImage}><Photo file={card.image} sizes="(max-width: 700px) 80vw, 25vw" /><span aria-hidden="true">↗</span></div><h3>{t.toolCards[index].title}</h3><p>{t.toolCards[index].description}</p></Link> : null)}</MarketingCarousel>
        </PageContainer></section>

        <section className={styles.useCasesSection}><PageContainer size="wide"><div className={styles.sectionIntro}><h2>{t.useTitle}</h2><p>{t.useDescription}</p><PrimaryButton href="/app" className={styles.darkButton}>{t.create} ↗</PrimaryButton></div><div className={styles.useCaseGrid}>{t.useCases.map((item, index) => <Link href="/app/inspiration" className={styles.useCaseCard} key={item.title}><Photo file={useCaseImages[index]} sizes="(max-width: 700px) 80vw, 25vw" /><div><h3>{item.title}</h3><p>{item.description}</p></div></Link>)}</div></PageContainer></section>

        <section className={styles.originalsSection} id="inspiration"><PageContainer size="wide"><div className={styles.splitHeading}><div><h2>{t.originalsTitle}</h2><p>{t.originalsDescription}</p></div><Link href="/app/inspiration" className={styles.textLink}>{t.originalsAction} ↗</Link></div><div className={styles.originalFeature} onKeyDown={event => { if (event.key === "ArrowLeft" || event.key === "ArrowRight") { event.preventDefault(); setOriginal((original + (event.key === "ArrowRight" ? 1 : 2)) % originalImages.length); } }}><Photo key={originalImages[original]} file={originalImages[original]} sizes="100vw" /><div><span>{brandName} · {t.preview}</span><h3>{t.originalTitles[original]}</h3><Link href="/app/inspiration" className={styles.outlineButton}>{t.originalsAction} ↗</Link></div></div><div className={styles.slideControls}><button type="button" onClick={() => setOriginal((original + 2) % 3)} aria-label={t.previous}>←</button><div className={styles.slideDots}>{originalImages.map((image, index) => <button key={image} type="button" aria-label={`${t.showSlide} ${index + 1}`} aria-pressed={original === index} onClick={() => setOriginal(index)} />)}</div><span aria-live="polite">0{original + 1} / 03</span><button type="button" onClick={() => setOriginal((original + 1) % 3)} aria-label={t.next}>→</button></div></PageContainer></section>

        <section className={styles.plansSection} id="plans"><PageContainer size="wide"><div className={styles.sectionIntro}><h2>{t.plansTitle}</h2></div><div className={styles.planGrid}><article><span className={styles.planLabel}>{brandName}</span><h3>{t.personalTitle}</h3><p>{t.personalDescription}</p><PrimaryButton href="/app" className={styles.darkButton}>{t.create} ↗</PrimaryButton></article><article><span className={styles.planLabel}>✦</span><h3>{t.teamTitle}</h3><p>{t.teamDescription}</p><a href="mailto:support@epifera.com" className={styles.outlineDarkButton}>{t.support} ↗</a></article></div><div className={styles.benefitGrid}>{t.benefits.map((item, index) => <article key={item.title}><span aria-hidden="true">{["✳", "◈", "↗"][index]}</span><h3>{item.title}</h3><p>{item.description}</p></article>)}</div></PageContainer></section>

        <section className={styles.faqSection} id="faq"><PageContainer size="wide"><div><h2>{t.faqTitle}</h2><a href="mailto:support@epifera.com" className={styles.textLink}>{t.support} ↗</a></div><FAQ items={t.faq} name="homepage-faq" /></PageContainer></section>
        <section className={styles.closing}><Photo file="start-2x.webp" sizes="100vw" /><div><BrandLogo size={52} /><h2>{t.closingTitle}</h2><PrimaryButton href="/app" className={styles.whiteButton}>{t.create} ↗</PrimaryButton></div></section>
      </main>
      {introOpen ? <MarketingDialog title={t.why} onClose={() => setIntroOpen(false)}><MediaVideo src="/assets/videos/marketing/creative-suite-image.webm" poster="/assets/images/marketing/product-shots.webp" controls autoPlay muted playsInline preload="metadata" fit="contain" width={1280} height={720} /><div className={styles.introAction}><PrimaryButton href="/app" className={styles.darkButton}>{t.create}</PrimaryButton></div></MarketingDialog> : null}
      <footer className={styles.footer}><PageContainer size="wide"><div className={styles.footerTop}><div className={styles.footerIdentity}><Link href="/" className={styles.brand} aria-label={brandName}><BrandLogo size={40}/><BrandName /></Link><p>{t.footerDescription}</p></div><MarketingFooterGroups /></div><LegalLinks/><div className={styles.footerBottom}><span>© {new Date().getUTCFullYear()} {brandName}. {t.copyright}</span><a href={`mailto:${supportEmail}`}>{supportEmail} <span aria-hidden="true">↗</span></a></div></PageContainer></footer>
    </div>
  );
}
