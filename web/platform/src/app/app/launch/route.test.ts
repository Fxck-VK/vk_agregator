import { beforeEach, expect, it, vi } from 'vitest';
vi.mock('server-only', () => ({}));
vi.mock('@/features/session/home-entry.server', () => ({ loadHomeEntrySession: vi.fn() }));
vi.mock('@/i18n/server', () => ({ getRequestLocale: async () => 'ru' }));
import { loadHomeEntrySession } from '@/features/session/home-entry.server';
import { GET } from './route';

beforeEach(() => vi.resetAllMocks());
const request = (query = '?model=nano_banana_pro') => new Request(`https://epifera.com/ru/app/launch${query}`);

it('sends guests to login while the existing proxy owns the launch return cookie', async () => {
  vi.mocked(loadHomeEntrySession).mockResolvedValue('guest');
  const response = await GET(request());
  expect(response.status).toBe(307);
  expect(response.headers.get('location')).toBe('https://epifera.com/ru/login');
  expect(response.headers.get('cache-control')).toBe('private, no-store');
  expect(response.headers.get('x-robots-tag')).toBe('noindex, nofollow');
});

it.each(['authenticated', 'refresh_required'] as const)('uses the selected workspace for %s sessions; workspace guards still verify access', async session => {
  vi.mocked(loadHomeEntrySession).mockResolvedValue(session);
  const expected = {
    gpt_image_2: '/app/chats?model=gpt_image_2',
    nano_banana_pro: '/app/chats?model=nano_banana_pro',
    video_kling_v3: '/app/chats?model=video_kling_v3',
    video_veo_3_1_fast: '/app/chats?model=video_veo_3_1_fast',
    seedream_5_0_lite: '/app/chats?model=seedream_5_0_lite',
    flux_2_pro: '/app/chats?model=flux_2_pro',
    suno_v6: '/app/music?model=suno_v6',
    gpt_4o_mini_tts: '/app/speech?model=gpt_4o_mini_tts',
  };
  for (const [id, target] of Object.entries(expected)) {
    const response = await GET(request(`?model=${id}`));
    expect(response.headers.get('location')).toBe(`https://epifera.com/ru${target}`);
  }
});

it.each(['', '?model=unknown', '?model=https://external.test', '?model=suno_v6&model=flux_2_pro', '?model=suno_v6&next=https://external.test'])('rejects unknown or ambiguous navigation without checking an account: %s', async query => {
  const response = await GET(request(query));
  expect(response.status).toBe(400);
  expect(response.headers.get('location')).toBeNull();
  expect(loadHomeEntrySession).not.toHaveBeenCalled();
});
