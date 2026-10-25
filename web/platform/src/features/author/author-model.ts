import type {AuthorMaterial} from '@/i18n/public/author-en';
import type {Locale} from '@/i18n/locales';

export const authorPageSize=12;
export function paginateAuthorMaterials(items:readonly AuthorMaterial[],input:unknown){
  const pages=Math.max(1,Math.ceil(items.length/authorPageSize));
  const numeric=typeof input==='number'&&Number.isInteger(input)&&input>0?input:
    typeof input==='string'&&/^[1-9]\d*$/.test(input)?Number(input):1;
  const page=Math.min(pages,Number.isFinite(numeric)?numeric:pages);
  const start=(page-1)*authorPageSize;
  return {page,pages,items:items.slice(start,start+authorPageSize),start:items.length?start+1:0,end:Math.min(start+authorPageSize,items.length),total:items.length};
}
const normalize=(value:string,locale:Locale)=>value.normalize('NFD').replace(/\p{M}/gu,'').toLocaleLowerCase(locale).replace(/\s+/g,' ').trim();
export function searchAuthorMaterials(items:readonly AuthorMaterial[],query:string,locale:Locale='en'){
  const words=normalize(query,locale).split(' ').filter(Boolean);
  return items.filter(item=>{const text=normalize(`${item.title} ${item.category}`,locale);return words.every(word=>text.includes(word));});
}
export function authorPageWindow(page:number,pages:number){
  const count=Math.min(5,pages),start=Math.max(1,Math.min(page-2,pages-count+1));
  return Array.from({length:count},(_,i)=>start+i);
}
