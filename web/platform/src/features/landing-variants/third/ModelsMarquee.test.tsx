import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { afterAll, afterEach, beforeAll, expect, it, vi } from 'vitest';
import { LocaleProvider } from '@/i18n/LocaleProvider';
import { homeEn } from '@/i18n/public/home-en';
import { HomeModels } from './HomeExplore';

vi.mock('next/navigation', () => ({ useRouter: vi.fn() }));
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
  render(<LocaleProvider locale="ru"><HomeModels copy={homeEn} /></LocaleProvider>);
  const region = screen.getByRole('region', { name: homeEn.models.label });
  const card = within(region).getByRole('link', { name: 'GPT Image' });
  return { region, card };
}

it('keeps one accessible set of models and removes the old arrow controls', () => {
  const { region } = mount();
  expect(within(region).queryAllByRole('button')).toHaveLength(0);
  expect(within(region).getAllByRole('link')).toHaveLength(homeEn.models.items.length);
  for (const [index, link] of within(region).getAllByRole('link').entries()) {
    expect(link).toHaveAttribute('href', `/ru/app/launch?model=${homeEn.models.items[index].id}`);
  }
});

it('reveals the photo while moving under the mouse and pauses only for keyboard focus', () => {
  const { region, card } = mount();
  expect(card).toHaveAttribute('data-preview', 'false');
  fireEvent.pointerEnter(card, { pointerType: 'mouse' });
  expect(region).toHaveAttribute('data-paused', 'false');
  expect(card).toHaveAttribute('data-preview', 'true');
  fireEvent.pointerLeave(region, { pointerType: 'mouse' });
  expect(region).toHaveAttribute('data-paused', 'false');
  expect(card).toHaveAttribute('data-preview', 'false');
  fireEvent.focus(card);
  expect(region).toHaveAttribute('data-paused', 'true');
  expect(card).toHaveAttribute('data-preview', 'true');
  fireEvent.blur(card, { relatedTarget: document.body });
  expect(region).toHaveAttribute('data-paused', 'false');
});

it('uses the first touch tap for the preview and dismisses it on an outside touch', () => {
  const { region, card } = mount();
  fireEvent.pointerDown(card, { pointerType: 'touch', clientX: 150 });
  fireEvent.pointerUp(card, { pointerType: 'touch', clientX: 150 });
  expect(fireEvent.click(card, { detail: 1 })).toBe(false);
  expect(card).toHaveAttribute('data-preview', 'true');
  expect(region).toHaveAttribute('data-paused', 'true');
  fireEvent.pointerDown(document.body, { pointerType: 'touch' });
  expect(card).toHaveAttribute('data-preview', 'false');
});

it.each(['touch', 'mouse'])('does not navigate or activate a photo after a %s drag', pointerType => {
  const { card } = mount();
  fireEvent.pointerDown(card, { pointerType, clientX: 180 });
  fireEvent.pointerMove(card, { pointerType, clientX: 100 });
  fireEvent.pointerUp(card, { pointerType, clientX: 70 });
  expect(fireEvent.click(card, { detail: 1 })).toBe(false);
  expect(card).toHaveAttribute('data-preview', 'false');
});

it('resumes after a canceled touch gesture without waiting for horizontal inertia', () => {
  const { region, card } = mount();
  fireEvent.pointerDown(card, { pointerType: 'touch', clientX: 180, clientY: 240 });
  expect(region).toHaveAttribute('data-paused', 'true');
  fireEvent.pointerCancel(card, { pointerType: 'touch' });
  expect(region).toHaveAttribute('data-paused', 'false');
  expect(fireEvent.click(card, { detail: 1 })).toBe(false);
  expect(card).toHaveAttribute('data-preview', 'false');
});

it.each(['scroll', 'resize'])('reconciles a stationary mouse on %s without relying on pointer boundary events', async event => {
  const { region, card } = mount();
  const hit = vi.spyOn(document, 'elementFromPoint').mockReturnValue(card);
  fireEvent.pointerMove(document.body, { pointerType: 'mouse', clientX: 160, clientY: 240 });
  fireEvent.pointerEnter(card, { pointerType: 'mouse', clientX: 160, clientY: 240 });
  expect(card).toHaveAttribute('data-preview', 'true');
  hit.mockReturnValue(document.body);
  fireEvent(window, new Event(event));
  await waitFor(() => expect(card).toHaveAttribute('data-preview', 'false'));
  expect(region).toHaveAttribute('data-paused', 'false');
  hit.mockReturnValue(card.querySelector('span'));
  fireEvent(window, new Event(event));
  await waitFor(() => expect(card).toHaveAttribute('data-preview', 'true'));
  expect(region).toHaveAttribute('data-paused', 'false');
  expect(hit).toHaveBeenLastCalledWith(160, 240);
});

it('keeps keyboard focus independent when a scroll moves the mouse off the strip', async () => {
  const { region, card } = mount();
  const hit = vi.spyOn(document, 'elementFromPoint').mockReturnValue(document.body);
  fireEvent.pointerMove(document.body, { pointerType: 'mouse', clientX: 160, clientY: 240 });
  fireEvent.pointerEnter(card, { pointerType: 'mouse', clientX: 160, clientY: 240 });
  fireEvent.focus(card);
  fireEvent.scroll(window);
  await waitFor(() => expect(hit).toHaveBeenCalled());
  expect(region).toHaveAttribute('data-paused', 'true');
  expect(card).toHaveAttribute('data-preview', 'true');
  fireEvent.blur(card, { relatedTarget: document.body });
  expect(region).toHaveAttribute('data-paused', 'false');
});
