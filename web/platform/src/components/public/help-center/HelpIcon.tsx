import {FileIcon} from "@/components/icons/FileIcon";
import {GridIcon} from "@/components/icons/GridIcon";
import {SupportIcon} from "@/components/icons/SupportIcon";
import type {HelpIconKind} from "./types";
export function HelpIcon({kind}:{kind:HelpIconKind}){
  if(kind==="file")return <FileIcon/>;if(kind==="grid")return <GridIcon/>;if(kind==="support")return <SupportIcon/>;
  return <svg aria-hidden="true" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">{kind==="home"?<path d="m4 11 8-7 8 7M6 10v10h12V10M10 20v-6h4v6"/>:kind==="book"?<><path d="M12 5c-3-2-6-2-9-1v15c3-1 6-1 9 1 3-2 6-2 9-1V4c-3-1-6-1-9 1Zm0 0v15"/></>:kind==="search"?<><circle cx="10" cy="10" r="6"/><path d="m15 15 5 5"/></>:kind==="play"?<path d="m8 4 12 8-12 8Z" fill="currentColor" stroke="none"/>:kind==="clock"?<><circle cx="12" cy="12" r="9"/><path d="M12 6v6l-3 3"/></>:<><path d="M4 15h3v6H4zM10 10h3v11h-3zM16 4h3v17h-3z"/></>}</svg>;
}
