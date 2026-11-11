import { expect, it } from 'vitest';
import { pageChoiceStore } from './page-choice-store';

it('keeps the current page control when an older page disposes and clears it on unmount', () => {
  const first = pageChoiceStore.register({ id: 'old', title: 'Old', value: 'a', options: [{ id: 'a', label: 'A' }], onChange() {} });
  const second = pageChoiceStore.register({ id: 'current', title: 'Current', value: 'b', options: [{ id: 'b', label: 'B' }], onChange() {} });
  first();
  expect(pageChoiceStore.getSnapshot()?.id).toBe('current');
  expect(pageChoiceStore.getServerSnapshot()).toBeNull();
  second();
  expect(pageChoiceStore.getSnapshot()).toBeNull();
});
