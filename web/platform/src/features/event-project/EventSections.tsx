"use client";
import Image from 'next/image';
import {LandingArrow,LandingToolIcon} from '@/components/public/landing/LandingPrimitives';
import {eventProjectEn as t} from '@/i18n/public/event-project-en';
import {eventProjectReference as media} from '@/i18n/public/event-project-reference-en';
import Link from '@/i18n/Link';
import styles from './EventProject.module.css';
import {usePublicCopy} from "@/i18n/public/use-public-copy";
import {translatePublicText} from "@/i18n/public/public-copy";
import {useLocale} from "@/i18n/LocaleProvider";



export function EventProgramme(){
  const {t} = usePublicCopy(publicCopySourcesForLocale);

  return <section id="schedule" className={styles.programme} aria-labelledby="programme-title">
    <div className={styles.centerIntro}><span>{t.programme.label}</span><h2 id="programme-title">{t.programme.title}</h2><p>{t.programme.description}</p></div>
    <div className={styles.programmeGrid}>{t.programme.items.map(item=><a key={item.title} href={item.href} data-event-programme>
      <span className={styles.programmeIcon}><LandingToolIcon name={item.icon}/></span><h3>{item.title}</h3><p className={styles.meta}>{item.meta}</p><p>{item.description}</p>
    </a>)}</div>
  </section>;
}

export function EventFilm({onTrailer}:{onTrailer:()=>void}){
  const publicCopyLocale=useLocale();

  const {t,media} = usePublicCopy(publicCopySourcesForLocale);

  return <section id="screening" className={styles.film} aria-labelledby="film-title">
    <span className={styles.filmLabel}>{t.film.label}</span><h2 id="film-title">{t.film.title}</h2><p className={styles.filmIntro}>{t.film.description}</p>
    <div className={styles.filmInner}><div className={styles.filmCover}>
      <Image src={media['candela-cover'].src} fill alt={translatePublicText("Candela reference film cover",publicCopyLocale)} sizes="(max-width:679px) 80vw,90vw"/>
      <button type="button" onClick={onTrailer}><span aria-hidden="true">▶</span>{t.film.action}</button>
    </div><div className={styles.filmNotes}>{t.film.notes.map(note=><p key={note}>{note}</p>)}</div><small>{t.film.credit}</small></div>
  </section>;
}

export function EventDiscussion(){
  const {t} = usePublicCopy(publicCopySourcesForLocale);

  return <section id="conversation" className={styles.discussion} aria-labelledby="discussion-title">
    <div><span className={styles.pill}>{t.discussion.label}</span><h2 id="discussion-title">{t.discussion.title}</h2><p className={styles.meta}>{t.discussion.meta}</p><p>{t.discussion.description}</p><a className={styles.darkButton} href={t.discussion.href}>{t.discussion.action}</a></div>
    <ul>{t.discussion.items.map(item=><li key={item.title} data-event-discussion><span><LandingToolIcon name="text"/></span><div><h3>{item.title}</h3><p>{item.description}</p></div></li>)}</ul>
  </section>;
}

export function EventFeatures(){
  const {t,media} = usePublicCopy(publicCopySourcesForLocale);

  return <>{t.features.map((item,index)=><section id={item.id} key={item.id} className={styles.feature} data-event-feature data-reverse={index===1||undefined} aria-labelledby={`${item.id}-title`}>
    <figure><Image src={media[item.image].src} width={media[item.image].width} height={media[item.image].height} alt={item.credit} sizes="(max-width:1000px) 95vw,50vw"/><figcaption>{item.credit}</figcaption></figure>
    <div><span className={styles.pill}>{item.label}</span><h2 id={`${item.id}-title`}>{item.title}</h2><p className={styles.meta}>{item.meta}</p><p>{item.description}</p><Link href={item.href} className={styles.darkButton}>{item.action}</Link></div>
  </section>)}</>;
}

export function EventJournal(){
  const {t} = usePublicCopy(publicCopySourcesForLocale);

  return <section className={styles.journal} aria-labelledby="journal-title">
    <div className={styles.centerIntro}><span>{t.journal.label}</span><h2 id="journal-title">{t.journal.title}</h2><p>{t.journal.description}</p></div>
    <div className={styles.journalGrid}><Link href={t.journal.href} data-event-journal><span>{t.journal.category}</span><h3>{t.journal.cardTitle}</h3><p>{t.journal.text}</p><strong>{t.journal.action}<LandingArrow/></strong></Link></div>
  </section>;
}

const publicCopySourcesForLocale={t,media};
