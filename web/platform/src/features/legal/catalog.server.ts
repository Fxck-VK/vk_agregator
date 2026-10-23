import "server-only";
import { createHash } from "node:crypto";
import rawSource from "../../../legal/catalog.json";
import type { LegalMetadata, LegalSlug } from "./model";

type SourceCatalog = {
 version:string; marketing_version:string; date:string; revision_hash:string;
 documents:Array<{slug:string;title:string;title_en:string;sections:Array<{title:string;paragraphs:string[];table:string[][]}>}>;
};
const source:SourceCatalog=rawSource;

export function createLegalCatalog(name:string,inn:string,publish:boolean){
 name=name.trim();inn=inn.trim();
 const configured=name.length>0&&Buffer.byteLength(name)<=256&&!/[\u0000-\u001f\u007f-\u009f]/u.test(name)&&/^\d{12}$/.test(inn)&&!/^[0]+$/.test(inn);
 const published=publish&&configured;
 const digest=(text:string)=>createHash("sha256").update(text).digest("hex");
 const lines=source.documents.flatMap(doc=>[doc.slug,doc.title,doc.title_en,...doc.sections.flatMap(section=>[section.title,...section.paragraphs,...section.table.flat()])]);
 if(digest(lines.join("\n"))!==source.revision_hash)throw new Error("Legal catalog source is not reviewed");
 const checksum=digest(`${source.version}\n${source.marketing_version}\n${source.date}\n${source.revision_hash}\n${name}\n${inn}\n${published}`);
 const metadata:LegalMetadata={published,version:source.version,checksum,marketing_version:source.marketing_version,marketing_checksum:digest(`${checksum}\n${source.marketing_version}`)};
 const resolve=(text:string)=>text.replaceAll("{NAME}",name).replaceAll("{INN}",inn);
 return {metadata,configured,date:source.date,documents:source.documents.map(doc=>({...doc,slug:doc.slug as LegalSlug,sections:doc.sections.map(section=>({...section,paragraphs:section.paragraphs.map(resolve)}))}))};
}
export function getLegalCatalog(){return createLegalCatalog(process.env.LEGAL_OPERATOR_NAME??"",process.env.LEGAL_OPERATOR_INN??"",process.env.LEGAL_DOCUMENTS_PUBLISHED==="true");}
