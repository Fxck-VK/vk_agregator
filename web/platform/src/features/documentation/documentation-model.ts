import {documentationArticles,type DocumentationArticle,type DocumentationBlock} from "@/i18n/public/documentation-en";
import type {Locale} from "@/i18n/locales";
import {localizePublicData} from "@/i18n/public/public-copy";
export const documentationPath="/ai/docs";
export const documentationArticlePath=(slug:string,base=documentationPath)=>`${base}/${slug}`;
export function findDocumentationArticle(slug:string,locale:Locale="en"){return localizePublicData(documentationArticles,locale).find(article=>article.slug===slug);}
export function documentationNeighbours(slug:string,locale:Locale="en",source:readonly DocumentationArticle[]=documentationArticles):{previous?:DocumentationArticle;next?:DocumentationArticle}{
  const articles=localizePublicData(source,locale),index=articles.findIndex(article=>article.slug===slug);
  return index<0?{}:{previous:articles[index-1],next:articles[index+1]};
}
function blockText(block:DocumentationBlock):string{
  if(block.type==="paragraph")return block.content.map(part=>typeof part==="string"?part:part.text).join(" ");
  if(block.type==="list")return block.items.join(" ");
  if(block.type==="steps")return block.items.map(item=>`${item.title} ${item.text}`).join(" ");
  if(block.type==="questions")return block.items.map(item=>`${item.question} ${item.answer}`).join(" ");
  return block.text;
}
export function searchDocumentation(query:string,locale:Locale="en",source:readonly DocumentationArticle[]=documentationArticles){
  const words=query.trim().toLocaleLowerCase(locale).split(/\s+/).filter(Boolean);
  return localizePublicData(source,locale).filter(article=>{const text=[article.title,article.description,article.introduction,...article.sections.flatMap(section=>[section.title,...section.blocks.map(blockText)])].join(" ").toLocaleLowerCase(locale);return words.every(word=>text.includes(word));});
}
