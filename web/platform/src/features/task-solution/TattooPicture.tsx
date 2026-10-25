import Image from "next/image";

export type TattooImage = { readonly file: string; readonly alt: string; readonly width: number; readonly height: number };
export function TattooPicture({ image, sizes, preload = false, decorative = false }: { image: TattooImage; sizes: string; preload?: boolean; decorative?: boolean }) {
  return <Image src={`/assets/images/tattoo-solution/${image.file}`} alt={decorative ? "" : image.alt} fill sizes={sizes} preload={preload} />;
}
