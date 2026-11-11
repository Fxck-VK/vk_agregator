import { fireEvent, render, screen, within } from '@testing-library/react';
import { expect, it, vi } from 'vitest';
import { existsSync } from 'node:fs';
import path from 'node:path';
import { assetPaths } from '@/assets/asset-paths';

vi.mock('server-only', () => ({}));
vi.mock('next/navigation', () => ({ useRouter: vi.fn() }));
vi.mock('@/features/release/ReleaseViewProvider', () => ({ useReleaseView: () => ({ view: 'dev', canSwitch: true, localSimulation: true, enabled: () => true, allowsPath: () => true }) }));
import { LocaleProvider } from '@/i18n/LocaleProvider';
import { contentSources, resolveContentPage } from '@/content/templates/catalog';
import { PublicHome } from './PublicHome';
import { homePhotoSource, modelPhotos, originalPhotos, platformPhotos, possibilityPhotos, suitePhotos } from './photos';

const page = resolveContentPage('/', { preview: true })!;
const copy = contentSources({ ...page, template: 'T01' }, 'ru').homeEn;
const mount = () => render(<LocaleProvider locale="ru"><PublicHome copy={copy} /></LocaleProvider>);
function photoSource(image: Element | null) {
  const url = new URL(image?.getAttribute('src') ?? '', 'http://localhost');
  return url.searchParams.get('url') ?? url.pathname;
}

it('opens the intro as a video-only overlay and restores keyboard focus when closed', () => {
  vi.stubGlobal('matchMedia', vi.fn(() => ({ matches: true, addEventListener() {}, removeEventListener() {} })));
  try {
    mount();
    const opener = screen.getByRole('button', { name: copy.why });
    opener.focus();
    fireEvent.click(opener);
    const dialog = screen.getByRole('dialog', { name: copy.why });
    expect(within(dialog).queryByRole('heading')).not.toBeInTheDocument();
    expect(within(dialog).queryByText(copy.suite.intro)).not.toBeInTheDocument();
    expect(within(dialog).queryByRole('link')).not.toBeInTheDocument();
    const video = dialog.querySelector('video')!;
    expect(video).toHaveAttribute('src', assetPaths.videos.workspace.howItWorks);
    expect(video).toHaveAttribute('controls');
    expect(video).toHaveAttribute('autoplay');
    expect(video).toHaveAttribute('playsinline');
    const close = within(dialog).getByRole('button', { name: copy.controls.close });
    expect(close).toHaveFocus();
    video.focus();
    fireEvent.keyDown(video, { key: 'Tab' });
    expect(close).toHaveFocus();
    fireEvent.keyDown(close, { key: 'Tab', shiftKey: true });
    expect(video).toHaveFocus();
    fireEvent.keyDown(video, { key: 'Escape' });
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(opener).toHaveFocus();
  } finally { vi.unstubAllGlobals(); }
});

it('retains the original section order and Russian content while using contributor photographs', () => {
  const before = JSON.stringify(copy);
  const { container } = mount();
  expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent(copy.hero.title);
  expect(screen.queryByRole('link', { name: copy.announcement })).not.toBeInTheDocument();
  expect(screen.queryByRole('link', { name: copy.hero.eyebrow })).not.toBeInTheDocument();
  expect(screen.queryByRole('button', { name: copy.controls.pause })).not.toBeInTheDocument();
  expect(screen.queryByRole('button', { name: copy.controls.play })).not.toBeInTheDocument();
  const header = within(screen.getByRole('banner'));
  expect(header.queryByRole('navigation')).not.toBeInTheDocument();
  expect(header.queryByRole('button', { name: copy.controls.openMenu })).not.toBeInTheDocument();
  expect(header.queryByRole('button', { name: copy.controls.search })).not.toBeInTheDocument();
  expect([...container.querySelectorAll('main > section')].map(section => section.id)).toEqual([
    '', 'creative-suite', 'models', 'workflow', 'tools', 'possibilities', 'originals', 'teams', 'plans', 'questions', '',
  ]);
  for (const image of container.querySelectorAll<HTMLImageElement>('img[data-landing-photo]')) {
    expect(photoSource(image)).toContain('/assets/images/home/');
  }
  expect(photoSource(container.querySelector('img[data-landing-photo="hero"]'))).toBe(homePhotoSource('hero'));
  expect(screen.getAllByRole('link', { name: copy.start })[0]).toHaveAttribute('href', '/ru/app');
  expect(JSON.stringify(copy)).toBe(before);
});

it('switches suite photos by mouse and keyboard and retains the gallery controls', async () => {
  const { container } = mount();
  const platforms = within(screen.getByRole('tablist', { name: copy.suite.tabsLabel }));
  expect(platforms.getAllByRole('tab')).toHaveLength(3);
  for (const index of [2, 3, 4]) expect(platforms.queryByRole('tab', { name: copy.suite.tabs[index] })).not.toBeInTheDocument();
  const categories = within(screen.getByRole('tablist', { name: copy.suite.categoriesLabel }));
  copy.suite.categories.filter(category => category.href !== '/app/inspiration').forEach((category, index) => {
    fireEvent.click(categories.getByRole('tab', { name: new RegExp(category.name) }));
    expect(photoSource(container.querySelector(`img[data-landing-photo="${suitePhotos[index]}"]`))).toBe(homePhotoSource(suitePhotos[index]));
  });
  fireEvent.keyDown(platforms.getByRole('tab', { name: copy.suite.tabs[0] }), { key: 'ArrowRight' });
  expect(platforms.getByRole('tab', { name: copy.suite.tabs[1] })).toHaveFocus();
  expect(photoSource(container.querySelector('img[data-landing-photo="platformWorkspace"]'))).toBe(homePhotoSource('platformWorkspace'));
  copy.suite.platforms.forEach((_platform, index) => {
    if ('upcoming' in _platform && _platform.upcoming) return;
    fireEvent.click(platforms.getByRole('tab', { name: copy.suite.tabs[index + 1] }));
    expect(container.querySelector(`img[data-landing-photo="${platformPhotos[index]}"]`)).toBeInTheDocument();
  });
  const inspirationTab = platforms.getByRole('tab', { name: copy.suite.tabs[5] });
  fireEvent.keyDown(inspirationTab, { key: 'ArrowRight' });
  expect(platforms.getByRole('tab', { name: copy.suite.tabs[0] })).toHaveFocus();
  fireEvent.keyDown(platforms.getByRole('tab', { name: copy.suite.tabs[0] }), { key: 'ArrowLeft' });
  expect(inspirationTab).toHaveFocus();
  expect(container.querySelector('img[data-landing-photo="platformInspiration"]')).toHaveAttribute('data-landing-photo', 'platformInspiration');
  expect(screen.queryByText(copy.controls.comingSoon)).not.toBeInTheDocument();
  for (const [index, slide] of copy.originals.slides.entries()) {
    fireEvent.click(screen.getByRole('button', { name: `${copy.controls.showSlide} ${index + 1}` }));
    expect(await screen.findByRole('heading', { name: slide.title })).toBeInTheDocument();
    expect(photoSource(container.querySelector(`img[data-landing-photo="${originalPhotos[index]}"]`))).toBe(homePhotoSource(originalPhotos[index]));
    fireEvent.transitionEnd(container.querySelector('[data-original-track]')!, { propertyName: 'transform' });
  }
});

it('assigns every existing content slot to a local contributor asset, including the visible replacement gallery frame', () => {
  expect(suitePhotos).toHaveLength(copy.suite.categories.length);
  expect(platformPhotos).toHaveLength(copy.suite.platforms.length);
  expect(possibilityPhotos).toHaveLength(copy.possibilities.cards.length);
  expect(originalPhotos).toHaveLength(copy.originals.slides.length);
  expect(Object.keys(modelPhotos)).toEqual(copy.models.items.map(model => model.id));
  for (const photo of ['hero', 'closing', 'overview', ...suitePhotos, ...platformPhotos, ...Object.values(modelPhotos), ...possibilityPhotos, ...originalPhotos] as const) {
    expect(existsSync(path.join('public', homePhotoSource(photo)))).toBe(true);
  }
  expect(homePhotoSource('originalPortrait')).toBe(homePhotoSource('suiteVideo'));
});

it('keeps scenario captions on their photographs without arrows or card navigation', () => {
  const { container } = mount();
  const section = container.querySelector('#possibilities')!;
  expect(within(section as HTMLElement).getAllByRole('link')).toHaveLength(1);
  copy.possibilities.cards.forEach((card, index) => {
    const photo = container.querySelector(`img[data-landing-photo="${possibilityPhotos[index]}"]`)!;
    expect(within(photo.parentElement!).getByRole('heading', { name: card.title })).toBeInTheDocument();
    expect(within(photo.parentElement!).getByText(card.description)).toBeInTheDocument();
    const article = photo.closest('article')!;
    expect(article).toBeInTheDocument();
    expect(article.querySelector('a,button,[tabindex],svg')).toBeNull();
  });
});

it.each([false, true])('returns to the top with motion preference respected (reduced=%s)', reduced => {
  const scroll = vi.spyOn(window, 'scrollTo').mockImplementation(() => {});
  vi.stubGlobal('matchMedia', vi.fn(() => ({ matches: reduced, addEventListener() {}, removeEventListener() {} })));
  try {
    mount();
    fireEvent.click(screen.getByRole('link', { name: copy.controls.backToTop }));
    expect(scroll).toHaveBeenCalledExactlyOnceWith({ top: 0, behavior: reduced ? 'auto' : 'smooth' });
  } finally {
    scroll.mockRestore();
    vi.unstubAllGlobals();
  }
});
