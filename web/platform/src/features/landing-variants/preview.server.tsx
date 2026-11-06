import 'server-only';
import { getRequestReleaseState } from '@/features/release/server';
import { notFound } from '@/i18n/server';
import { renderContentPage } from '@/content/templates/render';

/** Comparison pages require the existing private DEV boundary, including direct URLs. */
export async function assertLandingPreviewAccess() {
  const release = await getRequestReleaseState();
  if (!release.canSwitch || release.view !== 'dev') notFound();
}

export async function renderLandingVariant(variant: string) {
  await assertLandingPreviewAccess();
  if (variant === 'original') return renderContentPage('/');
  if (variant === 'friend') {
    const [{ PublicHome }, { homeEn }] = await Promise.all([
      import('./friend/PublicHome'), import('./friend/home-copy'),
    ]);
    return <PublicHome copy={homeEn} />;
  }
  notFound();
}
