import { fireEvent, render, screen, within } from '@testing-library/react';
import { expect, it, vi } from 'vitest';
import { LocaleProvider } from '@/i18n/LocaleProvider';
import { homeEn, homeMediaBriefsEn, homeSuiteLaunchLabelsEn } from './home-copy';
import { PublicHome } from './PublicHome';

vi.mock('next/navigation', () => ({ useRouter: vi.fn() }));
vi.mock('@/features/models/model-catalog-cache', () => ({
  loadModelCatalog: vi.fn(async () => ({ items: [{ id: 'nano_banana_pro', name: 'Nano Banana Pro' }] })),
  resetModelCatalogCacheForTests: vi.fn(),
}));
function home() { return render(<LocaleProvider locale="ru"><PublicHome copy={homeEn} /></LocaleProvider>); }

it('keeps the friend artwork and editorial header separate from the original T01', () => {
  const { container } = home();
  expect(screen.getByRole('img', { name: homeMediaBriefsEn.hero.title })).toHaveAttribute('src', '/assets/images/home/hero.webp');
  expect(container.querySelector('header')).toHaveAttribute('data-variant', 'editorial');
  expect(container.querySelector('img[src*="/assets/images/marketing/"]')).toBeNull();
  expect(screen.getAllByRole('heading', { level: 1 })).toHaveLength(1);
});

it('keeps the friend category controls and destinations working', () => {
  home();
  const categories = within(screen.getByRole('tablist', { name: homeEn.suite.categoriesLabel }));
  fireEvent.click(categories.getByRole('tab', { name: homeEn.suite.categories[1].name }));
  expect(screen.getByRole('link', { name: homeSuiteLaunchLabelsEn.video })).toHaveAttribute('href', '/ru/app/models');
  expect(screen.getByText(homeEn.suite.categories[1].description)).toBeVisible();
});

it('retains our release-aware search in the friend presentation', async () => {
  home();
  fireEvent.click(screen.getByRole('button', { name: homeEn.controls.search }));
  const dialog = screen.getByRole('dialog', { name: homeEn.controls.search });
  fireEvent.change(within(dialog).getByRole('searchbox'), { target: { value: 'нана банана про' } });
  expect(await within(dialog).findByRole('link', { name: /Nano Banana Pro/ })).toHaveAttribute('href', '/ru/app?model=nano_banana_pro');
});
