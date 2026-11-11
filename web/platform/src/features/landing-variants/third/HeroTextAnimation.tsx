"use client";

import { Fragment, useEffect, useRef, useState, type CSSProperties } from 'react';
import { useDictionary } from '@/i18n/LocaleProvider';
import { useReleaseView } from '@/features/release/ReleaseViewProvider';
import { pageChoiceStore } from '@/features/local-development/page-choice-store';
import { useHeroSceneClock } from './useHeroSceneClock';
import styles from './HeroTextAnimation.module.css';

type TextEffect = 'fade' | 'words' | 'spotlight';
const effects: TextEffect[] = ['fade', 'words', 'spotlight'];
const phraseDuration = 2000;

function AnimatedPhrases({ effect, phrases }: { effect: TextEffect; phrases: readonly string[] }) {
  const target = useRef<HTMLDivElement>(null);
  const { scene, running } = useHeroSceneClock(target, phrases.length, phraseDuration);
  return <div ref={target} className={styles.animation} style={{ '--phrase-duration': `${phraseDuration}ms` } as CSSProperties} aria-hidden="true" data-hero-text-effect={effect} data-hero-text-scene={scene} data-running={running}>
    {effect === 'spotlight' ? <div className={styles.phrases}>{phrases.map((phrase, index) => <p data-phrase data-active={scene === index} key={phrase}>{phrase}</p>)}</div>
      : <div className={styles.single}>{effect === 'fade' ? <p className={styles.fade} key={scene}>{phrases[scene]}</p>
        : <p className={styles.words} key={scene}>{phrases[scene].split(/\s+/).map((word, index) => <Fragment key={index}><span style={{ '--word-index': index } as CSSProperties}>{word}</span>{' '}</Fragment>)}</p>}</div>}
  </div>;
}

export function HeroTextAnimation({ phrases }: { phrases: readonly string[] }) {
  const { landingVariants: { heroAnimation: copy } } = useDictionary();
  const release = useReleaseView();
  const [effect, setEffect] = useState<TextEffect>('fade');
  useEffect(() => {
    if (!release.canSwitch || release.view !== 'dev') return;
    return pageChoiceStore.register({
      id: 'third-hero-text', title: copy.title, value: effect,
      description: copy.options.find(option => option.id === effect)?.description,
      options: copy.options,
      onChange: next => { if (effects.includes(next as TextEffect)) setEffect(next as TextEffect); },
    });
  }, [copy, effect, release.canSwitch, release.view]);
  return phrases.length ? <AnimatedPhrases key={effect} effect={effect} phrases={phrases} /> : null;
}
