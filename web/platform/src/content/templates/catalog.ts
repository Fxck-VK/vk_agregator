import 'server-only';
import rawPages from '../pages/catalog.json';
import {additionalContentPages} from '../pages/generated';
import {templateDefaults} from './defaults';
import {templateDefinitions,type TemplateId} from './registry';
import {safeContentPath,validateContentPages} from './validation';
import {localizePublicData} from '@/i18n/public/public-copy';
import type {ContentPage,TemplateSources} from './types';
import type {PublicPagePublication,PublicPageSource} from '../publication/types';
import type {Locale} from '@/i18n/locales';

export const contentPages:readonly ContentPage[]=validateContentPages([...rawPages,...additionalContentPages],templateDefaults);
export function contentSources<K extends TemplateId>(page:ContentPage&{template:K},locale:Locale='ru'):TemplateSources<K>{
  const data:Record<string,unknown>={...templateDefaults[page.template],...page.content};
  const owner=templateDefinitions.find(t=>t.id===page.template);
  const child=('lessonSlug'in page&&page.lessonSlug)||('articleSlug'in page&&page.articleSlug);
  const identityPath=child?page.path.slice(0,page.path.lastIndexOf('/')):page.path;
  const identityId=child?page.id.split('--')[0]:page.id;
  if(owner&&identityPath!==owner.path){
    const keys:Record<string,string[]>={T10:['courseEn'],T12:['productCampaignWorkflow'],T13:['cinematicCollection'],T14:['flowerExample'],T15:['natureCategory'],T16:['gypsophilaMaterial'],T17:['mcpIntegration'],T20:['footballCaseStudy'],T22:['eventProject']};
    for(const key of keys[page.template]??[]){const item=data[key] as Record<string,unknown>;data[key]={...item,...('id'in item?{id:identityId}:{}),...('slug'in item?{slug:identityPath.split('/').at(-1)}:{}),...('path'in item?{path:identityPath}:{})};}
  }
  return localizePublicData(data,locale) as TemplateSources<K>;
}
export type ResolvedContentPage=ContentPage&{lessonSlug?:string;articleSlug?:string};
const indexes=new WeakMap<readonly ContentPage[],{all:ResolvedContentPage[];byPath:Map<string,ResolvedContentPage>}>();

/** Child lessons/help articles are derived from their parent's data, never hand-maintained routes. */
export function expandContentPages(pages:readonly ContentPage[]):ResolvedContentPage[]{
  const cached=indexes.get(pages);if(cached)return cached.all;
  const all=pages.flatMap(page=>{
    if(page.template==='T10'){
      const {courseEn}=contentSources({...page,template:'T10'});
      return [page,...courseEn.modules.flatMap(m=>m.lessons.map(lesson=>({...page,id:`${page.id}--${lesson.slug}`,path:`${page.path}/${lesson.slug}`,seo:{title:`${lesson.title} | ${courseEn.title}`,description:lesson.goal},lessonSlug:lesson.slug})))];
    }
    if(page.template==='T11'){
      const {documentationArticles}=contentSources({...page,template:'T11'});
      return [page,...documentationArticles.map(article=>({...page,id:`${page.id}--${article.slug}`,path:`${page.path}/${article.slug}`,seo:{title:article.title,description:article.description},articleSlug:article.slug}))];
    }
    return [page];
  });
  const byPath=new Map<string,ResolvedContentPage>(),ids=new Set<string>();
  for(const page of all){if(!safeContentPath(page.path)||byPath.has(page.path)||ids.has(page.id))throw new Error('Invalid or duplicate derived content URL or ID.');byPath.set(page.path,page);ids.add(page.id);}
  indexes.set(pages,{all,byPath});return all;
}
export function canReadContent(page:ContentPage,{preview=false,now=Date.now()}={}){
  if(page.status==='archived')return false;
  if(page.status==='published')return !page.publishedAt||Date.parse(page.publishedAt)<=now;
  return preview||templateDefinitions.some(t=>t.id===page.template&&t.path===page.path);
}
export function resolveContentPage(path:string,options?:{preview?:boolean;now?:number},pages:readonly ContentPage[]=contentPages){
  expandContentPages(pages);
  const found=indexes.get(pages)!.byPath.get(path);
  // Existing sample children retain the same draft visibility as their parent.
  if(!found)return undefined;
  const parent=found.lessonSlug||found.articleSlug?pages.find(p=>p.id===found.id.split('--')[0])??found:found;
  return canReadContent(parent,options)?found:undefined;
}
export function contentPublication(page:ContentPage):PublicPagePublication{
  return {id:page.id,status:page.status,indexing:page.indexing,paths:{ru:page.path},...(page.publishedAt?{publishedAt:page.publishedAt}:{}),...(page.updatedAt?{updatedAt:page.updatedAt}:{})};
}
export function contentPublicationSources(existing:readonly PublicPageSource[],pages:readonly ContentPage[]=contentPages):PublicPageSource[]{
  const expanded=expandContentPages(pages),paths=new Set(expanded.map(p=>p.path));
  return [
    ...existing.map(source=>({...source,pages:source.pages.filter(page=>!page.paths.ru||!paths.has(page.paths.ru))})),
    ...expanded.map(page=>({route:page.path==='/'?'/':'/[...missing]',pages:[contentPublication(page)]})),
  ];
}
