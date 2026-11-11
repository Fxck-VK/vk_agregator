import { act, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { useHeroSceneClock } from './useHeroSceneClock';

let visibility: (visible: boolean) => void;
let reduced = false;
let motionChange: () => void;
beforeEach(() => {
  vi.useFakeTimers();
  reduced = false;
  vi.spyOn(document, 'hidden', 'get').mockReturnValue(false);
  vi.stubGlobal('matchMedia', () => ({ get matches() { return reduced; }, addEventListener(_event: string, cb: () => void) { motionChange = cb; }, removeEventListener() {} }));
  vi.stubGlobal('IntersectionObserver', class {
    constructor(cb: (entries: { isIntersecting: boolean }[]) => void) { visibility = visible => cb([{ isIntersecting: visible }]); }
    observe() {} disconnect() {}
  });
});
afterEach(() => { vi.restoreAllMocks(); vi.unstubAllGlobals(); vi.useRealTimers(); });
const mount = () => { const target = { current: document.body }; return renderHook(() => useHeroSceneClock(target, 4, 1000)); };

it('cycles visible scenes, resets the delay on selection and clears its timer on unmount', () => {
  const { result, unmount } = mount();
  act(() => visibility(true));
  act(() => vi.advanceTimersByTime(2000));
  expect(result.current.scene).toBe(2);
  act(() => result.current.select(0));
  act(() => vi.advanceTimersByTime(999));
  expect(result.current.scene).toBe(0);
  act(() => vi.advanceTimersByTime(1));
  expect(result.current.scene).toBe(1);
  act(() => vi.advanceTimersByTime(3000));
  expect(result.current.scene).toBe(0);
  unmount();
  expect(vi.getTimerCount()).toBe(0);
});

it('stops offscreen, when the document is hidden and when reduced motion is enabled', () => {
  const { result } = mount();
  act(() => visibility(true));
  act(() => visibility(false));
  act(() => vi.advanceTimersByTime(5000));
  expect(result.current.scene).toBe(0);
  act(() => visibility(true));
  vi.spyOn(document, 'hidden', 'get').mockReturnValue(true);
  act(() => document.dispatchEvent(new Event('visibilitychange')));
  act(() => vi.advanceTimersByTime(5000));
  expect(result.current.scene).toBe(0);
  vi.spyOn(document, 'hidden', 'get').mockReturnValue(false);
  reduced = true;
  act(() => motionChange());
  act(() => vi.advanceTimersByTime(5000));
  expect(result.current.running).toBe(false);
  act(() => result.current.select(3));
  expect(result.current.scene).toBe(3);
});
