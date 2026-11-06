import type { Metadata } from 'next';
import type { ReactNode } from 'react';
import { assertLandingPreviewAccess } from '@/features/landing-variants/preview.server';
import { getRequestDictionary } from '@/i18n/server';
import { brandName } from '@/config/brand';

export async function generateMetadata(): Promise<Metadata> {
  const { landingVariants: copy } = await getRequestDictionary();
  return { title: `${copy.title} · ${brandName}`, robots: { index: false, follow: false } };
}
export const dynamic = 'force-dynamic';

export default async function LandingVariantsLayout({ children }: { children: ReactNode }) {
  await assertLandingPreviewAccess();
  return children;
}
