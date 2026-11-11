"use client";

import Image from 'next/image';
import { homePhotoSource, type HomePhoto } from './photos';

/** Third-variant media reuses the contributor's local photographs. */
export function HomePicture({ photo, alt = '', sizes = '(max-width: 767px) 92vw, 45vw', preload = false, original = false, className }: {
  photo: HomePhoto; alt?: string; sizes?: string; preload?: boolean; original?: boolean; className?: string;
}) {
  return <Image src={homePhotoSource(photo)} data-landing-photo={photo} alt={alt} fill sizes={sizes} preload={preload} unoptimized={original} className={className} />;
}
