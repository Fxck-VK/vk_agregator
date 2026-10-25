import {brandFilePrefix,brandName} from "@/config/brand";
import type {PublicExampleImage} from "./public-example-en";
import {botanicalReferenceImages as images,botanicalVideoStills} from "./material-detail-reference-en";
export type StockItem={
 readonly id:string;readonly slug:string;readonly path:string;readonly title:string;
 readonly mediaType:"photo";readonly media:PublicExampleImage;readonly publicMediaIds:readonly string[];
 readonly creatorId:string;readonly creator:{name:string;url:string};readonly provenance:{source:string;url:string};
 readonly licenseRef:string;readonly rights:{status:"source-terms-unverified";note:string};
 readonly fileDescriptor:{width:number;height:number;format:"webp";bytes:number};readonly downloadPolicy:"reference-preview-only";
 readonly categoryIds:readonly string[];readonly taxonIds:readonly string[];readonly sourceAI:false;
 readonly suggestedPrompt:string;readonly photos:readonly PublicExampleImage[];readonly designs:readonly PublicExampleImage[];readonly videoStills:readonly PublicExampleImage[];
 readonly downloads:readonly {label:string;src:string;filename:string;mime:string}[];
};
export const materialDetailPath="/stock/items/dried-gypsophila/gypsophila-01";
export const gypsophilaMaterial:StockItem={
 id:"gypsophila-01",slug:"dried-gypsophila",path:materialDetailPath,title:"Dried gypsophila with window shadow on a beige wall",
 mediaType:"photo",media:images[0],publicMediaIds:[...images,...botanicalVideoStills].map(item=>item.id),creatorId:"rawpixel-com",creator:{name:"rawpixel.com",url:"https://www.magnific.com/author/rawpixel-com"},
 provenance:{source:"Magnific",url:"https://www.magnific.com/free-photo/dried-gypsophila-with-window-shadow-beige-wall_19134237.htm"},
 licenseRef:"https://www.magnific.com/profile/license/pdf/19134237?lang=en",rights:{status:"source-terms-unverified",note:"Commercial rights are not granted by this preview. Check the source license before reusing this photograph."},
 fileDescriptor:{width:images[0].width,height:images[0].height,format:"webp",bytes:651014},downloadPolicy:"reference-preview-only",categoryIds:["nature"],taxonIds:["botanical","shadow","flower","beige","wallpaper","white"],sourceAI:false,
 suggestedPrompt:"A minimalist botanical photograph of delicate dried gypsophila branches against a warm beige plaster wall. Soft natural sunlight from a window casts long geometric shadows. Airy composition, fine cream-colored blossoms, subtle wall texture, gentle depth of field and a calm neutral palette. No text or watermarks.",
 photos:images.slice(1,13),designs:images.slice(13),videoStills:botanicalVideoStills,
 downloads:[{label:"WebP preview",src:images[0].src,filename:`${brandFilePrefix}-dried-gypsophila-preview.webp`,mime:"image/webp"},{label:"PNG preview",src:"/assets/images/material-detail/gypsophila-01-preview.png",filename:`${brandFilePrefix}-dried-gypsophila-preview.png`,mime:"image/png"}],
};
export const materialDetailEn={
 metadata:{title:`Dried Gypsophila Photo — Botanical Reference | ${brandName}`,description:"Explore a minimalist dried gypsophila photograph, its source, preview downloads and related botanical references."},
 current:"Photo reference",currentShort:"Photo",skip:"Skip to photo",about:"About this photo",source:"Source creator",license:"Usage and license",dimensions:"Preview dimensions",format:"Preview file",rights:"Commercial rights are not granted by this preview.",original:"Only the cached preview is available here. The source lists an original 4667 × 7000 JPEG; that original is not included in these downloads.",
 originNote:"Create your own images with the",generator:"AI Image Generator",hint:"Start a new image from this visual direction.",hintDetail:"The reference photo is not automatically attached.",
 photos:"You might also like these photos",designs:"Related vectors",psd:"Related PSD",videos:"Related video stills",videoNote:"These are still previews. Video playback is not available in this reference archive.",designNote:"Raster previews only; layered PSD and vector source files are not available here.",category:"Explore Nature Photos",filter:"Explore related tags",all:"Show all related images",
 presets:[{label:"Warmer light",instruction:"Use warm golden-hour sunlight and softer window shadows."},{label:"Minimal",instruction:"Use fewer branches, more negative space and a clean minimalist composition."},{label:"Variations",instruction:"Explore a fresh arrangement of dried flowers and different window shadows."}],
} as const;
