import { expect, it } from 'vitest';
import { normalizeMarqueeOffset } from './useModelMarquee';

it.each([0, 999, 1000, 1999, 2000, 3005, -7])('wraps offset %s into the middle copy without changing the visible phase', offset => {
  const cycle = 1000;
  const wrapped = normalizeMarqueeOffset(offset, cycle);
  expect(wrapped).toBeGreaterThanOrEqual(cycle);
  expect(wrapped).toBeLessThan(2 * cycle);
  expect(((wrapped - offset) % cycle + cycle) % cycle).toBe(0);
});
it('keeps the initial offset while a hidden or unmeasured group has no width', () => {
  expect(normalizeMarqueeOffset(12, 0)).toBe(12);
});
