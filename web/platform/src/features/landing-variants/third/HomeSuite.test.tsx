import { fireEvent, render, screen, within } from '@testing-library/react';
import { expect, it, vi } from 'vitest';
import { LocaleProvider } from '@/i18n/LocaleProvider';
import { homeEn } from '@/i18n/public/home-en';
import { HomeSuite } from './HomeSuite';
import { suitePhotos } from './photos';

const visibleCategories = homeEn.suite.categories.filter(category => category.href !== '/app/inspiration');

vi.mock('next/navigation', () => ({ useRouter: vi.fn() }));
vi.mock('@/features/release/ReleaseViewProvider', () => ({ useReleaseView: () => ({ view: 'dev', canSwitch: true, localSimulation: true, enabled: () => true, allowsPath: () => true }) }));
function mount() {
  const onTour = vi.fn();
  const { container } = render(<LocaleProvider locale="ru"><HomeSuite copy={homeEn} onTour={onTour} /></LocaleProvider>);
  return { container, onTour, list: screen.getByRole('tablist', { name: homeEn.suite.categoriesLabel }) };
}

it('exposes a vertical category list with only the selected description and media active', () => {
  const { container, list } = mount();
  expect(list).toHaveAttribute('aria-orientation', 'vertical');
  expect(within(list).getAllByRole('tab')).toHaveLength(4);
  expect(within(list).queryByRole('tab', { name: homeEn.suite.categories[4].name })).not.toBeInTheDocument();
  visibleCategories.forEach((category, index) => {
    const tab = within(list).getByRole('tab', { name: category.name });
    expect(tab).toHaveAttribute('aria-selected', String(index === 0));
    const description = within(tab).getByText(category.description).closest('[aria-hidden]');
    expect(description).toHaveAttribute('aria-hidden', String(index !== 0));
  });
  const active = container.querySelectorAll('[data-suite-frame][data-active="true"]');
  expect(active).toHaveLength(1);
  expect(active[0].querySelector('img')).toHaveAttribute('data-landing-photo', suitePhotos[0]);
});

it('switches the description, image, launch link and selected indicator together', () => {
  const { container, list } = mount();
  visibleCategories.forEach((category, index) => {
    const tab = within(list).getByRole('tab', { name: category.name });
    fireEvent.click(tab);
    expect(tab).toHaveAttribute('aria-selected', 'true');
    expect(container.querySelectorAll('[data-suite-frame][data-active="true"]')).toHaveLength(1);
    expect(container.querySelector('[data-suite-frame][data-active="true"] img')).toHaveAttribute('data-landing-photo', suitePhotos[index]);
    expect(within(tab).getByText(category.description).closest('[aria-hidden]')).toHaveAttribute('aria-hidden', 'false');
    expect(screen.getByRole('link', { name: `${homeEn.explore} ${category.name}` })).toHaveAttribute('href', `/ru${category.href}`);
  });
});

it('wraps vertical keyboard selection and preserves the chosen category across platform tabs', () => {
  const { container, list, onTour } = mount();
  const tabs = within(list).getAllByRole('tab');
  fireEvent.keyDown(tabs[0], { key: 'ArrowUp' });
  expect(tabs.at(-1)).toHaveFocus();
  expect(tabs.at(-1)).toHaveAttribute('aria-selected', 'true');
  fireEvent.keyDown(tabs.at(-1)!, { key: 'ArrowDown' });
  expect(tabs[0]).toHaveFocus();
  fireEvent.keyDown(tabs[0], { key: 'End' });
  expect(tabs.at(-1)).toHaveFocus();
  const platforms = within(screen.getByRole('tablist', { name: homeEn.suite.tabsLabel })).getAllByRole('tab');
  expect(platforms).toHaveLength(3);
  fireEvent.click(platforms[1]);
  expect(container.querySelector('[data-suite-platform] img')).toHaveAttribute('data-landing-photo', 'platformWorkspace');
  fireEvent.click(platforms[0]);
  expect(within(screen.getByRole('tablist', { name: homeEn.suite.categoriesLabel })).getAllByRole('tab').at(-1)).toHaveAttribute('aria-selected', 'true');
  fireEvent.click(screen.getByRole('button', { name: homeEn.suite.tabs[1] }));
  expect(onTour).toHaveBeenCalledOnce();
});

it('keeps one moving platform indicator mounted while mouse and keyboard selection change', () => {
  mount();
  const list = screen.getByRole('tablist', { name: homeEn.suite.tabsLabel });
  const tabs = within(list).getAllByRole('tab');
  const indicator = within(list).getByTestId('mode-switch-panel-indicator');
  expect(indicator).toHaveAttribute('aria-hidden', 'true');
  fireEvent.click(tabs[1]);
  expect(tabs[1]).toHaveAttribute('aria-selected', 'true');
  expect(within(list).getByTestId('mode-switch-panel-indicator')).toBe(indicator);
  fireEvent.keyDown(tabs[1], { key: 'End' });
  expect(tabs[2]).toHaveFocus();
  expect(tabs[2]).toHaveAttribute('aria-selected', 'true');
  expect(within(list).getByTestId('mode-switch-panel-indicator')).toBe(indicator);
  expect(document.getElementById(tabs[2].getAttribute('aria-controls')!)).toHaveAttribute('aria-labelledby', tabs[2].id);
});

it('retains the three surfaces for a stable crossfade while making only the selected content interactive', () => {
  const { container } = mount();
  const tabs = within(screen.getByRole('tablist', { name: homeEn.suite.tabsLabel })).getAllByRole('tab');
  const surfaces = [...container.querySelectorAll('[data-suite-surface]')];
  expect(surfaces).toHaveLength(3);
  tabs.forEach((tab, index) => {
    fireEvent.click(tab);
    expect([...container.querySelectorAll('[data-suite-surface]')]).toEqual(surfaces);
    surfaces.forEach((surface, position) => {
      expect(surface).toHaveAttribute('aria-hidden', String(position !== index));
      expect(surface.hasAttribute('inert')).toBe(position !== index);
    });
    const title = index === 0 ? homeEn.suite.heading : homeEn.suite.platforms[index === 1 ? 0 : 4].title;
    expect(screen.getAllByRole('heading', { level: 3 })).toHaveLength(1);
    expect(screen.getByRole('heading', { name: title })).toBeInTheDocument();
  });
});
