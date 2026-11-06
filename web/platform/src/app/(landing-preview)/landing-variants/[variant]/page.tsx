import { renderLandingVariant } from '@/features/landing-variants/preview.server';

export default async function LandingVariantPage({ params }: { params: Promise<{ variant: string }> }) {
  return renderLandingVariant((await params).variant);
}
