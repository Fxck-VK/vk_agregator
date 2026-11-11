import { fireEvent, render, screen, within } from '@testing-library/react';
import { afterAll, afterEach, beforeAll, expect, it, vi } from 'vitest';
import { LocaleProvider } from '@/i18n/LocaleProvider';
import { homeEn } from '@/i18n/public/home-en';
import { HomeTools } from './HomeExplore';

vi.mock('next/navigation', () => ({ useRouter: vi.fn() }));
vi.mock('@/features/release/ReleaseViewProvider', () => ({ useReleaseView: () => ({ view: 'dev', canSwitch: true, localSimulation: true, enabled: () => true, allowsPath: () => true }) }));
const nativeElementFromPoint = document.elementFromPoint;
beforeAll(() => {
  Object.defineProperty(document, 'elementFromPoint', { configurable: true, writable: true, value: () => null });
  vi.stubGlobal('PointerEvent', class extends MouseEvent {
  readonly pointerType: string;
  constructor(type: string, init: PointerEventInit = {}) { super(type, init); this.pointerType = init.pointerType ?? 'mouse'; }
  });
});
afterEach(() => vi.restoreAllMocks());
afterAll(() => {
  vi.unstubAllGlobals();
  Object.defineProperty(document, 'elementFromPoint', { configurable: true, writable: true, value: nativeElementFromPoint });
});

function mount() {
  render(<LocaleProvider locale="ru"><HomeTools copy={homeEn} /></LocaleProvider>);
  return screen.getByRole('region', { name: homeEn.tools.label });
}

it('has no category tabs or arrows and exposes the full set of original tool links once', () => {
  const region = mount();
  expect(screen.queryByRole('tablist')).not.toBeInTheDocument();
  expect(screen.queryByRole('tabpanel')).not.toBeInTheDocument();
  expect(region).toHaveAttribute('data-tool-marquee');
  expect(within(region).queryAllByRole('button')).toHaveLength(0);
  expect(within(region).getAllByRole('link')).toHaveLength(homeEn.tools.items.length + 1);
  for (const tool of homeEn.tools.items) expect(within(region).getByRole('link', { name: `${tool.title} ${tool.description}` })).toHaveAttribute('href', `/ru${tool.href}`);
});

it('keeps moving during hover and pauses for keyboard focus and a touch gesture', () => {
  const region = mount();
  const link = within(region).getAllByRole('link')[0];
  fireEvent.pointerEnter(region, { pointerType: 'mouse', clientX: 30, clientY: 30 });
  expect(region).toHaveAttribute('data-paused', 'false');
  fireEvent.pointerLeave(region, { pointerType: 'mouse' });
  expect(region).toHaveAttribute('data-paused', 'false');
  fireEvent.focus(link);
  expect(region).toHaveAttribute('data-paused', 'true');
  fireEvent.blur(link, { relatedTarget: document.body });
  expect(region).toHaveAttribute('data-paused', 'false');
  fireEvent.pointerDown(link, { pointerType: 'touch', clientX: 180 });
  expect(region).toHaveAttribute('data-paused', 'true');
  fireEvent.pointerMove(link, { pointerType: 'touch', clientX: 90 });
  fireEvent.pointerCancel(link, { pointerType: 'touch' });
  expect(region).toHaveAttribute('data-paused', 'false');
  expect(fireEvent.click(link, { detail: 1 })).toBe(false);
});

it('keeps moving under a stationary pointer when the page scrolls or resizes', () => {
  const region = mount();
  fireEvent.pointerEnter(region, { pointerType: 'mouse', clientX: 120, clientY: 200 });
  expect(region).toHaveAttribute('data-paused', 'false');
  fireEvent.scroll(window);
  expect(region).toHaveAttribute('data-paused', 'false');
  fireEvent.resize(window);
  expect(region).toHaveAttribute('data-paused', 'false');
});
