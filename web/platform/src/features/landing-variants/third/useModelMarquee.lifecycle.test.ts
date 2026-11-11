import { act, renderHook } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { useModelMarquee } from './useModelMarquee';

afterEach(() => vi.unstubAllGlobals());

function fixture(reduced = false) {
  vi.stubGlobal('matchMedia', vi.fn(() => ({ matches: reduced, addEventListener: vi.fn(), removeEventListener: vi.fn() })));
  let groupWidth = 100;
  const viewport = document.createElement('div');
  const group = document.createElement('div');
  Object.defineProperty(viewport, 'clientWidth', { value: 900 });
  group.getBoundingClientRect = () => new DOMRect(0, 0, groupWidth, 307);
  const hook = renderHook(({ contentKey }) => useModelMarquee(true, contentKey), { initialProps: { contentKey: 'initial' } });
  act(() => { hook.result.current.viewport.current = viewport; hook.result.current.original.current = group; });
  hook.rerender({ contentKey: 'first' });
  return { ...hook, viewport, setWidth: (width: number) => { groupWidth = width; } };
}

it('measures a replaced filter before paint and restarts at the first complete copy', () => {
  const { result, rerender, viewport, setWidth } = fixture();
  expect(result.current.ready).toBe(true);
  expect(viewport.scrollLeft).toBe(100);
  viewport.scrollLeft = 150;
  setWidth(200);
  rerender({ contentKey: 'second' });
  expect(viewport.scrollLeft).toBe(200);
  expect(result.current.copies).toBe(8);
});

it('keeps enough repeated groups to fill a wide viewport with reduced motion', () => {
  const { result } = fixture(true);
  expect(result.current.reduced).toBe(true);
  expect(result.current.copies).toBe(12);
});
