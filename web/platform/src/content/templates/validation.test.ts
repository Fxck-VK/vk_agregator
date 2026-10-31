import {describe,expect,it,vi} from 'vitest';
vi.mock('server-only',()=>({}));
import raw from '../pages/catalog.json';
import {templateDefaults} from './defaults';
import {validateContentPages} from './validation';
import {canReadContent,contentSources,expandContentPages,resolveContentPage,contentPublicationSources} from './catalog';
import type {ContentPage} from './types';
import {buildPublicSitemap} from '@/i18n/seo';
import {templateDefinitions} from './registry';

const base=()=>structuredClone(raw) as ContentPage[];
function article(){const page=base().find(p=>p.template==='T09')!;page.id='new-article';page.path='/blog/new-article';page.content={artPromptsEn:JSON.parse(JSON.stringify(templateDefaults.T09.artPromptsEn))};return page;}
describe('public content validation and publication',()=>{
  it.each(templateDefinitions)('admits the complete Russian authoring example for $id',definition=>{
    const page=base().find(p=>p.template===definition.id)!;
    page.content=definition.managed?{}:JSON.parse(JSON.stringify(contentSources(page)));
    expect(()=>validateContentPages([page],templateDefaults)).not.toThrow();
  });
  it('admits the 24 existing records and derives all lesson/help paths',()=>{
    const pages=validateContentPages(base(),templateDefaults);expect(pages).toHaveLength(24);expect(expandContentPages(pages)).toHaveLength(45);
  });
  it('adds a second article from data, retaining the shared source without mutation',()=>{
    const page=article(),copy=page.content.artPromptsEn as {title:string};copy.title='Как снять товар';
    const pages=validateContentPages([...base(),page],templateDefaults);
    const resolved=resolveContentPage(page.path,{preview:true},pages)!;
    expect(contentSources({...resolved,template:'T09'}).artPromptsEn.title).toBe('Как снять товар');
    expect(templateDefaults.T09.artPromptsEn.title).not.toBe(copy.title);
    expect(resolveContentPage(page.path,{preview:false},pages)).toBeUndefined();
  });
  it('keeps the parent course identity on a derived lesson page',()=>{
    const page=base().find(p=>p.template==='T10')!;page.id='another-course';page.path='/academy/courses/another-course';
    page.content={courseEn:JSON.parse(JSON.stringify(templateDefaults.T10.courseEn))};
    const pages=validateContentPages([page],templateDefaults),lesson=expandContentPages(pages).find(p=>p.lessonSlug)!;
    expect(contentSources({...lesson,template:'T10'}).courseEn.slug).toBe('another-course');
  });
  it.each([
    ['missing id',(p:ContentPage)=>{delete (p as Partial<ContentPage>).id;}],
    ['null id',(p:ContentPage)=>{Object.assign(p,{id:null});}],
    ['numeric id',(p:ContentPage)=>{Object.assign(p,{id:123});}],
    ['duplicate id',(p:ContentPage)=>{p.id=raw[0].id;}],
    ['duplicate URL',(p:ContentPage)=>{p.path='/ai';}],
    ['service takeover',(p:ContentPage)=>{p.path='/app/other';}],
    ['English publication',(p:ContentPage)=>{p.locale='en' as 'ru';}],
    ['unknown source',(p:ContentPage)=>{p.content.bad={};}],
    ['null required source',(p:ContentPage)=>{p.content.artPromptsEn=null;}],
    ['array instead of source',(p:ContentPage)=>{p.content.artPromptsEn=[];}],
    ['null nested object',(p:ContentPage)=>{(p.content.artPromptsEn as Record<string,unknown>).metadata=null;}],
    ['array instead of nested object',(p:ContentPage)=>{(p.content.artPromptsEn as Record<string,unknown>).metadata=[];}],
    ['missing body',(p:ContentPage)=>{delete (p.content.artPromptsEn as Record<string,unknown>).sections;}],
    ['unknown block',(p:ContentPage)=>{(p.content.artPromptsEn as {lead:unknown[]}).lead=[{type:'html',html:'<script/>'}];}],
    ['unreviewed release',(p:ContentPage)=>{p.status='published';p.publishedAt='2026-10-29T00:00:00Z';}],
    ['malformed date',(p:ContentPage)=>{p.updatedAt='tomorrow';}],
    ['missing relation',(p:ContentPage)=>{p.relatedIds=['missing'];}],
  ])('rejects %s',(_label,mutate)=>{const page=article();mutate(page);expect(()=>validateContentPages([...base(),page],templateDefaults)).toThrow();});
  it('rejects unsafe media/links and managed billing overrides',()=>{
    const page=article();(page.content.artPromptsEn as {breadcrumbs:{href?:string}[]}).breadcrumbs[0].href='javascript:alert(1)';
    expect(()=>validateContentPages([...base(),page],templateDefaults)).toThrow('Unsafe');
    const pricing=base();pricing.find(p=>p.template==='T18')!.content={pricingEn:JSON.parse(JSON.stringify(templateDefaults.T18.pricingEn))};
    expect(()=>validateContentPages(pricing,templateDefaults)).toThrow('Managed');
  });
  it.each([null,[]])('rejects a non-object primary source without relying on renderer errors',value=>{
    const page=base().find(p=>p.template==='T02')!;page.content={aiToolsEn:value};
    expect(()=>validateContentPages([page],templateDefaults)).toThrow('Incompatible content field');
  });
  it('derives robots/sitemap admission from status/date and removes archive',()=>{
    const page=article();page.status='published';page.indexing='index';page.publishedAt='2026-10-29T00:00:00Z';page.reviewedAt=page.publishedAt;
    const now=Date.parse('2026-10-30T00:00:00Z');
    expect(canReadContent(page,{now})).toBe(true);
    expect(buildPublicSitemap(contentPublicationSources([],validateContentPages([page],templateDefaults)),now).map(p=>p.url)).toEqual([expect.stringMatching(/\/ru\/blog\/new-article$/)]);
    expect(canReadContent(page,{now:0})).toBe(false);
    page.status='archived';expect(canReadContent(page,{preview:true,now})).toBe(false);expect(buildPublicSitemap(contentPublicationSources([],[page]),now)).toEqual([]);
  });
});
