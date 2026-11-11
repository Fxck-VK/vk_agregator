import { cleanup, fireEvent, render, screen, within } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { LocaleProvider } from '@/i18n/LocaleProvider';
import { homeEn } from '@/i18n/public/home-en';
import { subscriptionPreviewPlans } from '@/features/pricing/subscription-preview';
import { SubscriptionPlansDialog } from '@/components/layout/WorkspaceHeader/SubscriptionPlansDialog';
import { HomePlans } from './HomeFinale';

const { push } = vi.hoisted(() => ({ push: vi.fn() }));
vi.mock('next/navigation', () => ({ useRouter: () => ({ push }) }));
vi.mock('@/features/release/ReleaseViewProvider', () => ({ useReleaseView: () => ({ enabled: () => false, allowsPath: () => true }) }));
afterEach(() => { cleanup(); vi.clearAllMocks(); });

it('reuses the exact workspace tariff cards with canonical prices, credits and discount badges', () => {
  const landing = render(<LocaleProvider locale="ru"><HomePlans copy={homeEn} /></LocaleProvider>);
  const cards = screen.getAllByRole('article');
  expect(cards).toHaveLength(5);
  subscriptionPreviewPlans.forEach((plan, index) => {
    expect(within(cards[index]).getByRole('heading', { name: plan.name })).toBeVisible();
    expect(cards[index]).toHaveTextContent(String(plan.amount / 100));
    expect(cards[index]).toHaveTextContent(new RegExp(String(plan.stars).split('').join('\\s*')));
  });
  for (const discount of ['−13%', '−16%', '−23%', '−28%']) expect(screen.getByText(discount)).toBeVisible();
  expect(landing.container.querySelector('[data-color-scheme="dark"]')).toBeInTheDocument();
  const markup = cards.map(card => card.outerHTML);
  landing.unmount();
  render(<LocaleProvider locale="ru"><SubscriptionPlansDialog onClose={() => {}} /></LocaleProvider>);
  expect(screen.getAllByRole('article').map(card => card.outerHTML)).toEqual(markup);
});

it('takes every landing subscription action through the existing protected workspace entry', () => {
  render(<LocaleProvider locale="ru"><HomePlans copy={homeEn} /></LocaleProvider>);
  const actions = screen.getAllByRole('button', { name: 'Активировать подписку' });
  expect(actions).toHaveLength(5);
  actions.forEach(action => fireEvent.click(action));
  expect(push.mock.calls).toEqual(Array.from({ length: 5 }, () => ['/ru/app']));
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
});
