import {brandName} from '@/config/brand';
import type {Locale} from '@/i18n/locales';
import {formatMessage,type MessageParameters} from '@/i18n/format';
import sources from './translations/sources.json';
import russian from './translations/ru.json';

export const protectedPublicCopyKeys=new Set([
  'id','slug','href','url','src','file','poster','key','value','type','kind',
  'mode','tier','currency','period','group','formats','extensions',
  'executionMode','modelId','model_id','operation','operationId','assetId',
  'ownerId','ownerKind','publicId','categoryId','icon','tone','mark','color',
  'className','indexing','direction','orientation',
  'taxonId','taxonIds','relatedCategoryIds','categoryIds','orderedMemberIds',
  'license','people','mediaType','downloadPolicy','origin','memberType',
  'taskIds','publicMediaIds','sortPolicy','modelVersionId','mime','filename','path','moderationState',
]);
const normalizeBrand=(text:string,name:string)=>name?text.split(name).join('{brand}'):text;
const ru=russian as Record<string,string>;
const translations=new Map(Object.entries(sources).filter(([id])=>ru[id]!==undefined).map(([id,source])=>[normalizeBrand(source,'Epifera'),normalizeBrand(ru[id],'Epifera')]));

export function translatePublicText(text:string,locale:Locale):string{
  if(locale!=='ru')return text;
  const translated=translations.get(normalizeBrand(text,brandName));
  return translated===undefined?text:translated.split('{brand}').join(brandName);
}

export function translatePublicMessage(text:string,locale:Locale,parameters:MessageParameters):string{
  return formatMessage(translatePublicText(text,locale),parameters);
}

const russianObjects=new WeakMap<object,object>();
/** Explicit request locale; immutable data projection, never DOM rewriting. */
export function localizePublicData<T>(value:T,locale:Locale):T{
  if(locale!=='ru')return value;
  function project(item:unknown,key?:string):unknown{
    if(key&&protectedPublicCopyKeys.has(key))return item;
    if(key==='status'&&typeof item==='string'&&/^[a-z][a-z0-9_-]*$/.test(item))return item;
    if(key==='support'&&typeof item==='string'&&['supported','unsupported','unspecified'].includes(item))return item;
    if(typeof item==='string')return translatePublicText(item,locale);
    if(item===null||typeof item!=='object')return item;
    const cached=russianObjects.get(item);if(cached)return cached;
    if(Array.isArray(item)){
      const output:unknown[]=[];russianObjects.set(item,output);
      item.forEach(child=>output.push(project(child)));return output;
    }
    const output:Record<string,unknown>={};russianObjects.set(item,output);
    for(const[childKey,child]of Object.entries(item)){
      // Media tags are canonical taxonomy IDs. Editorial tags (e.g. course labels)
      // remain text; media consumers translate their labels separately.
      output[childKey]=childKey==='tags'&&'src' in item?child:project(child,childKey);
    }
    return output;
  }
  return project(value) as T;
}
