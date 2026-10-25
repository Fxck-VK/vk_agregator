"use client";

import Image from "next/image";
export type ReferenceImage = { readonly file: string; readonly alt: string };
export function GeneratorPicture({ image, className, preload = false, sizes = "(max-width: 820px) 92vw, 40vw", decorative = false }: { image: ReferenceImage; className?: string; preload?: boolean; sizes?: string; decorative?: boolean }) {
  return <Image src={`/assets/images/image-generator/${image.file}`} alt={decorative ? "" : image.alt} fill sizes={sizes} preload={preload} className={className} />;
}
