import type {Locale} from "@/i18n/locales";
import {translatePublicText} from "@/i18n/public/public-copy";

// Canonical taxonomy values remain suitable for filtering and bookmarks.
// Only the human label is translated, separately from its identifier.
const labels:Readonly<Record<string,string>>={
  nature:"Nature",forest:"Forest",mountains:"Mountains",lakes:"Lakes",flowers:"Flowers",rivers:"Rivers",sunset:"Sunset",meadows:"Meadows",wildlife:"Wildlife",
  botanical:"Botanical",shadow:"Shadow",flower:"Flower",beige:"Beige",wallpaper:"Wallpaper",white:"White",aesthetic:"Aesthetic",pastel:"Pastel",pattern:"Pattern",
  cinematic:"Cinematic",city:"City",people:"People",sea:"Sea",lake:"Lake",mountain:"Mountain",portrait:"Portrait photography",landscape:"Nature landscape",night:"Night",train:"Train",leaf:"Leaf","video-still":"Video still",photo:"Photo",
};
export function publicTagLabel(tag:string,locale:Locale):string{
  return locale==="en"?tag:translatePublicText(labels[tag]??tag,locale);
}
