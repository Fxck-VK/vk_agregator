import Image from "next/image";
import type {EditorialImage} from "./types";
export function EditorialPicture({image,sizes,preload=false,decorative=false}:{image:EditorialImage;sizes:string;preload?:boolean;decorative?:boolean}) {
  return <Image src={image.src} fill alt={decorative?"":image.alt} sizes={sizes} preload={preload}/>;
}
