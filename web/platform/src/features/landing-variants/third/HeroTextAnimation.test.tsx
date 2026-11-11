import { act, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { LocaleProvider } from '@/i18n/LocaleProvider';
import { homeEn } from '@/i18n/public/home-en';
import { pageChoiceStore } from '@/features/local-development/page-choice-store';
import { HeroTextAnimation } from './HeroTextAnimation';

vi.mock('@/features/release/ReleaseViewProvider', () => ({ useReleaseView: () => ({ canSwitch: true, view: 'dev' }) }));
beforeEach(() => vi.stubGlobal('matchMedia', () => ({ matches: true, addEventListener() {}, removeEventListener() {} })));
afterEach(() => { vi.unstubAllGlobals(); vi.restoreAllMocks(); vi.useRealTimers(); });

it('renders text only and lets the development panel switch all three effects', () => {
  const { container, unmount } = render(<LocaleProvider locale="ru"><HeroTextAnimation phrases={homeEn.hero.capabilities} /></LocaleProvider>);
  expect(container.querySelector('img,video,canvas,button')).toBeNull();
  expect(screen.queryByRole('tablist')).not.toBeInTheDocument();
  expect(pageChoiceStore.getSnapshot()?.title).toBe('Анимация текста');
  for (const id of ['fade', 'words', 'spotlight']) {
    act(() => pageChoiceStore.getSnapshot()!.onChange(id));
    expect(container.querySelector('[data-hero-text-effect]')).toHaveAttribute('data-hero-text-effect', id);
    expect(pageChoiceStore.getSnapshot()?.value).toBe(id);
    expect(container.querySelector('[data-hero-text-scene]')).toHaveAttribute('data-hero-text-scene', '0');
  }
  expect(container.querySelectorAll('[data-phrase]')).toHaveLength(homeEn.hero.capabilities.length);
  unmount();
  expect(pageChoiceStore.getSnapshot()).toBeNull();
});

it('changes the visible phrase every two seconds', () => {
  vi.useFakeTimers();
  vi.spyOn(document, 'hidden', 'get').mockReturnValue(false);
  vi.stubGlobal('matchMedia', () => ({ matches: false, addEventListener() {}, removeEventListener() {} }));
  let show: () => void = () => {};
  vi.stubGlobal('IntersectionObserver', class {
    constructor(cb: (entries: { isIntersecting: boolean }[]) => void) { show = () => cb([{ isIntersecting: true }]); }
    observe() {} disconnect() {}
  });
  const { container, unmount } = render(<LocaleProvider locale="ru"><HeroTextAnimation phrases={homeEn.hero.capabilities} /></LocaleProvider>);
  act(show);
  act(() => vi.advanceTimersByTime(1999));
  expect(container.querySelector('[data-hero-text-scene]')).toHaveAttribute('data-hero-text-scene', '0');
  act(() => vi.advanceTimersByTime(1));
  expect(container.querySelector('[data-hero-text-scene]')).toHaveAttribute('data-hero-text-scene', '1');
  act(() => vi.advanceTimersByTime(2000));
  expect(container.querySelector('[data-hero-text-scene]')).toHaveAttribute('data-hero-text-scene', '2');
  unmount();
});
