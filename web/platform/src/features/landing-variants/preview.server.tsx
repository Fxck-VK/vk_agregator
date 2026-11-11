import 'server-only';
import { getRequestReleaseState } from '@/features/release/server';
import { getRequestLocale, notFound } from '@/i18n/server';
import { TemplateDataProvider } from '@/content/templates/TemplateDataProvider';

/** Comparison pages require the existing private DEV boundary, including direct URLs. */
export async function assertLandingPreviewAccess() {
  const release = await getRequestReleaseState();
  if (!release.canSwitch || release.view !== 'dev') notFound();
}

export async function renderLandingVariant(variant: string) {
  await assertLandingPreviewAccess();
  if (variant === 'friend') {
    const [{ PublicHome }, { homeEn }] = await Promise.all([
      import('./friend/PublicHome'), import('./friend/home-copy'),
    ]);
    return <PublicHome copy={homeEn} />;
  }
  if (variant === 'original' || variant === 'third') {
    const [{ PublicHome }, { resolveContentPage, contentSources }] = await Promise.all([
      variant === 'original' ? import('@/features/public-home/PublicHome') : import('./third/PublicHome'),
      import('@/content/templates/catalog'),
    ]);
    const page = resolveContentPage('/', { preview: true });
    if (!page || page.template !== 'T01') notFound();
    const data = contentSources({ ...page, template: 'T01' }, await getRequestLocale());
    return <TemplateDataProvider data={{ ...data, __route: { path: '/', basePath: '' } }}>
      <PublicHome copy={data.homeEn} />
    </TemplateDataProvider>;
  }
  notFound();
}
