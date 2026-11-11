import { beforeEach, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';

const state = vi.hoisted(() => ({ view: 'dev', canSwitch: true, localSimulation: true }));
vi.mock('server-only', () => ({}));
vi.mock('@/features/release/server', () => ({ getRequestReleaseState: async () => state }));
vi.mock('@/i18n/server', () => ({ getRequestLocale: async () => 'ru', notFound: () => { throw new Error('NOT_FOUND'); } }));
vi.mock('@/features/public-home/PublicHome', () => ({ PublicHome: () => <main>Сохранённая исходная главная</main> }));
vi.mock('./friend/PublicHome', () => ({ PublicHome: ({ copy }: { copy: { hero: { title: string } } }) => <main>{copy.hero.title}</main> }));
vi.mock('./third/PublicHome', () => ({ PublicHome: ({ copy }: { copy: { hero: { title: string } } }) => <main>{copy.hero.title}</main> }));
import '@/content/templates/catalog';
import { assertLandingPreviewAccess, renderLandingVariant } from './preview.server';

beforeEach(() => Object.assign(state, { view: 'dev', canSwitch: true, localSimulation: true }));

it.each([
  { view: 'prod', canSwitch: false, localSimulation: false },
  { view: 'prod', canSwitch: true, localSimulation: true },
  { view: 'dev', canSwitch: false, localSimulation: false },
])('denies previews without trusted DEV access: %j', async value => {
  Object.assign(state, value);
  await expect(assertLandingPreviewAccess()).rejects.toThrow('NOT_FOUND');
  await expect(renderLandingVariant('friend')).rejects.toThrow('NOT_FOUND');
  await expect(renderLandingVariant('third')).rejects.toThrow('NOT_FOUND');
});

it('allows protected DEV without requiring local simulation', async () => {
  state.localSimulation = false;
  await expect(assertLandingPreviewAccess()).resolves.toBeUndefined();
});

it('keeps the original variant independent of the promoted canonical renderer', async () => {
  render(await renderLandingVariant('original'));
  expect(screen.getByText('Сохранённая исходная главная')).toBeInTheDocument();
});

it('renders the separately preserved Russian friend variant', async () => {
  render(await renderLandingVariant('friend'));
  expect(screen.getByRole('main')).toHaveTextContent(/платформа/i);
});

it('renders the isolated third variant with unchanged Russian T01 content', async () => {
  expect(await renderLandingVariant('third')).toMatchObject({
    props: { data: { homeEn: {
      hero: { title: expect.stringMatching(/Творческая платформа/) },
      suite: { categories: [expect.objectContaining({ image: 'product-shots.webp' }), ...Array(4).fill(expect.anything())] },
    } } },
  });
});

it('does not accept arbitrary import paths', async () => {
  for (const id of ['../public-home', '']) {
    await expect(renderLandingVariant(id)).rejects.toThrow('NOT_FOUND');
  }
});
