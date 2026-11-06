import { beforeEach, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';

const state = vi.hoisted(() => ({ view: 'dev', canSwitch: true, localSimulation: true }));
vi.mock('server-only', () => ({}));
vi.mock('@/features/release/server', () => ({ getRequestReleaseState: async () => state }));
vi.mock('@/i18n/server', () => ({ notFound: () => { throw new Error('NOT_FOUND'); } }));
vi.mock('@/content/templates/render', () => ({ renderContentPage: vi.fn(async () => <main>Исходный SEO-шаблон</main>) }));
vi.mock('./friend/PublicHome', () => ({ PublicHome: ({ copy }: { copy: { hero: { title: string } } }) => <main>{copy.hero.title}</main> }));
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
});

it('allows protected DEV without requiring local simulation', async () => {
  state.localSimulation = false;
  await expect(assertLandingPreviewAccess()).resolves.toBeUndefined();
});

it('uses the existing SEO renderer for the original variant', async () => {
  render(await renderLandingVariant('original'));
  expect(screen.getByText('Исходный SEO-шаблон')).toBeInTheDocument();
});

it('renders the separately preserved Russian friend variant', async () => {
  render(await renderLandingVariant('friend'));
  expect(screen.getByRole('main')).toHaveTextContent(/платформа/i);
});

it('does not invent a third variant or accept arbitrary import paths', async () => {
  for (const id of ['third', '../public-home', '']) {
    await expect(renderLandingVariant(id)).rejects.toThrow('NOT_FOUND');
  }
});
