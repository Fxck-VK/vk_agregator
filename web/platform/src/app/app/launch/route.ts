import { NextResponse } from 'next/server';
import { modelEntryTarget } from '@/features/models/model-entry';
import { loadHomeEntrySession } from '@/features/session/home-entry.server';
import { getRequestLocale } from '@/i18n/server';
import { localizeHref } from '@/i18n/routing';

export const dynamic = 'force-dynamic';
const headers = { 'Cache-Control': 'private, no-store', 'X-Robots-Tag': 'noindex, nofollow' };

/** The proxy saves this safe /app return path on navigation, never on prefetch. */
export async function GET(request: Request): Promise<Response> {
  const query = new URL(request.url).searchParams;
  const target = query.size === 1 ? modelEntryTarget(query.get('model') ?? '') : null;
  if (!target) return new Response(null, { status: 400, headers });
  const session = await loadHomeEntrySession();
  // Refresh-only sessions enter the existing guarded workspace recovery flow.
  const href = session === 'guest' ? '/login' : target;
  const response = NextResponse.redirect(new URL(localizeHref(href, await getRequestLocale()), request.url), 307);
  for (const [key, value] of Object.entries(headers)) response.headers.set(key, value);
  return response;
}
