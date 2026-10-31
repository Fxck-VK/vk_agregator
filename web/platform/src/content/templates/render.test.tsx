import {afterEach,beforeEach,describe,expect,it,vi} from 'vitest';
import {cleanup,render} from '@testing-library/react';
import {LocaleProvider} from '@/i18n/LocaleProvider';
// Preload component modules outside timed assertions; production rendering remains lazy.
import '@/features/public-home/PublicHome';
import '@/features/ai-tools-overview/ToolsOverview';
import '@/features/image-generator-landing/ImageGeneratorLanding';
import '@/features/model-family/KlingFamily';
import '@/features/model-version/Kling30Landing';
import '@/features/task-solution/TattooSolution';
import '@/features/audience/MarketingAudience';
import '@/features/comparison/AvatarComparison';
import '@/features/editorial-article/ArtPromptsArticle';
import '@/features/academy/AcademyCourse';
import '@/features/documentation/DocumentationCenter';
import '@/features/workflow/WorkflowLanding';
import '@/features/collection/ExampleCollection';
import '@/features/public-example/PublicExamplePage';
import '@/features/material-category/MaterialCategoryPage';
import '@/features/material-detail/MaterialDetailPage';
import '@/features/integration/IntegrationLanding';
import '@/features/pricing/PricingLanding';
import '@/features/business/BusinessLanding';
import '@/features/customer-story/CustomerStory';
import '@/features/about/AboutLanding';
import '@/features/event-project/EventLanding';
import '@/features/author/AuthorLanding';
vi.mock('server-only',()=>({}));
vi.mock('@/i18n/server',async()=>{
  const {getDictionary}=await import('@/i18n/dictionary');
  return {getRequestLocale:async()=>'ru',getRequestDictionary:async()=>getDictionary('ru'),notFound:()=>{throw new Error('NOT_FOUND');}};
});
vi.mock('@/i18n/navigation',async importOriginal=>({...await importOriginal<object>(),usePathname:()=>'/route',useRouter:()=>({push:vi.fn(),replace:vi.fn(),back:vi.fn(),refresh:vi.fn(),prefetch:vi.fn()})}));
import {contentPages,contentSources} from './catalog';
import {renderContentRecord} from './render';
import {templateDefinitions} from './registry';
import type {ContentPage,Json} from './types';

function headings(value:Json,marker:string,key=''):Json{
  if(typeof value==='string')return /^(title|heading|headline|name$|line)/i.test(key)?`${marker} ${value}`:value;
  if(Array.isArray(value))return value.map(item=>headings(item,marker,key));
  if(value&&typeof value==='object')return Object.fromEntries(Object.entries(value).map(([k,v])=>[k,headings(v,marker,k)]));
  return value;
}
beforeEach(()=>{vi.spyOn(HTMLMediaElement.prototype,'play').mockResolvedValue();vi.spyOn(HTMLMediaElement.prototype,'pause').mockImplementation(()=>{});});
afterEach(()=>{cleanup();vi.restoreAllMocks();});
describe('all 24 data-driven renderers preserve their existing layout',()=>{
  it.each(templateDefinitions)('$id renders different records through the same component',async definition=>{
    const sample=contentPages.find(p=>p.template===definition.id)!;
    const output:string[]=[];
    for(const marker of ['Первая запись','Вторая запись']){
      const page:ContentPage={...sample,content:definition.managed?{}:headings(JSON.parse(JSON.stringify(contentSources(sample))) as Json,marker) as Record<string,Json>};
      const view=await renderContentRecord(page),{container}=render(<LocaleProvider locale="ru">{view}</LocaleProvider>);
      const heading=container.querySelector('h1');expect(heading,definition.id).not.toBeNull();
      if(!definition.managed)expect(heading!.textContent,definition.id).toContain(marker);
      output.push(container.querySelector('main')?.className??'');
      cleanup();
    }
    expect(output[0]).toBe(output[1]);
  });
  it('uses the new article body and media in the outline, main text and cover',async()=>{
    const sample=contentPages.find(p=>p.template==='T09')!,data=JSON.parse(JSON.stringify(contentSources({...sample,template:'T09'})));
    data.artPromptsEn.title='Новая предметная фотография';
    data.artPromptsEn.lead=[{type:'paragraph',content:['Собственное описание новой статьи.']}];
    data.artPromptsEn.sections[0].title='Новый раздел';data.artPromptsEn.sections[0].label='Новый раздел';
    data.artPromptsReferenceEn.hero={...data.artPromptsReferenceEn.hero,src:data.artPromptsReferenceEn.illustrations[0].src,alt:'Новая обложка'};
    const view=await renderContentRecord({...sample,id:'new-body',path:'/blog/new-body',content:data});
    const {container}=render(<LocaleProvider locale="ru">{view}</LocaleProvider>);
    expect(container.querySelector('article')?.textContent).toContain('Собственное описание новой статьи.');
    expect(container.querySelector('aside nav')?.textContent??container.querySelector('aside')?.textContent).toContain('Новый раздел');
    expect(container.querySelector('img[alt="Новая обложка"]')).not.toBeNull();
  });
  it('filters the new category members rather than the original reference catalogue',async()=>{
    const sample=contentPages.find(p=>p.template==='T15')!,data=JSON.parse(JSON.stringify(contentSources({...sample,template:'T15'})));
    data.natureMaterials=data.natureMaterials.slice(0,1);data.initialMaterialFilters.query='';
    data.natureMaterials[0].alt='Предметная фотография товара';data.natureMaterials[0].tags=['product'];
    const view=await renderContentRecord({...sample,id:'one-material',path:'/photos/one-material',content:data});
    const {container}=render(<LocaleProvider locale="ru">{view}</LocaleProvider>);
    expect(container.querySelectorAll('[data-material-id], [data-material-card]')).toHaveLength(1);
  });
});
