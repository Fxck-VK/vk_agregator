import 'server-only';
import type {ReactNode} from 'react';
import {TemplateDataProvider} from './TemplateDataProvider';
import {contentSources,resolveContentPage,contentPublication,type ResolvedContentPage} from './catalog';
import type {TemplateId} from './registry';
import type {TemplateSources} from './types';
import {getRequestLocale,notFound} from '@/i18n/server';
import {publicPageMetadata} from '@/i18n/seo';
import {paginateAuthorMaterials} from '@/features/author/author-model';

function lookup(path:string){return resolveContentPage(path,{preview:process.env.NODE_ENV==='development'});}
export async function contentMetadata(path:string){
  const page=lookup(path);if(!page)notFound();
  return {...publicPageMetadata(await getRequestLocale(),contentPublication(page)),...page.seo};
}
export async function renderContentPage(path:string,searchPage?:string|string[]){
  const page=lookup(path);if(!page)notFound();return renderContentRecord(page,searchPage);
}

/** Every branch reuses the original page's component; no second JSX layout. */
export async function renderContentRecord(page:ResolvedContentPage,searchPage?:string|string[]){
  const data=contentSources(page,await getRequestLocale());
  const source=<K extends TemplateId>(id:K)=>{void id;return data as TemplateSources<K>;};
  let view:ReactNode;
  switch(page.template){
    case 'T01':{const {PublicHome}=await import('@/features/public-home/PublicHome');view=<PublicHome copy={source('T01').homeEn}/>;break;}
    case 'T02':{const {ToolsOverview}=await import('@/features/ai-tools-overview/ToolsOverview');view=<ToolsOverview/>;break;}
    case 'T03':{const {ImageGeneratorLanding}=await import('@/features/image-generator-landing/ImageGeneratorLanding');view=<ImageGeneratorLanding/>;break;}
    case 'T04':{const {KlingFamily}=await import('@/features/model-family/KlingFamily');view=<KlingFamily/>;break;}
    case 'T05':{const {Kling30Landing}=await import('@/features/model-version/Kling30Landing');view=<Kling30Landing/>;break;}
    case 'T06':{const {TattooSolution}=await import('@/features/task-solution/TattooSolution');view=<TattooSolution/>;break;}
    case 'T07':{const {MarketingAudience}=await import('@/features/audience/MarketingAudience');view=<MarketingAudience/>;break;}
    case 'T08':{const {AvatarComparison}=await import('@/features/comparison/AvatarComparison');view=<AvatarComparison/>;break;}
    case 'T09':{const {ArtPromptsArticle}=await import('@/features/editorial-article/ArtPromptsArticle');view=<ArtPromptsArticle/>;break;}
    case 'T10':{const {AcademyCourse}=await import('@/features/academy/AcademyCourse');const course=source('T10').courseEn;const lesson=course.modules.flatMap(m=>m.lessons.map(l=>({...l,moduleId:m.id,moduleTitle:m.title}))).find(l=>l.slug===page.lessonSlug);view=<AcademyCourse lesson={lesson}/>;break;}
    case 'T11':{const {DocumentationCenter}=await import('@/features/documentation/DocumentationCenter');view=<DocumentationCenter article={source('T11').documentationArticles.find(a=>a.slug===page.articleSlug)}/>;break;}
    case 'T12':{const {WorkflowLanding}=await import('@/features/workflow/WorkflowLanding');view=<WorkflowLanding workflow={source('T12').productCampaignWorkflow}/>;break;}
    case 'T13':{const {ExampleCollection}=await import('@/features/collection/ExampleCollection');view=<ExampleCollection collection={source('T13').cinematicCollection}/>;break;}
    case 'T14':{const {PublicExamplePage}=await import('@/features/public-example/PublicExamplePage');view=<PublicExamplePage example={source('T14').flowerExample}/>;break;}
    case 'T15':{const {MaterialCategoryPage}=await import('@/features/material-category/MaterialCategoryPage');view=<MaterialCategoryPage/>;break;}
    case 'T16':{const {MaterialDetailPage}=await import('@/features/material-detail/MaterialDetailPage');view=<MaterialDetailPage material={source('T16').gypsophilaMaterial}/>;break;}
    case 'T17':{const {IntegrationLanding}=await import('@/features/integration/IntegrationLanding');view=<IntegrationLanding/>;break;}
    case 'T18':{const {PricingLanding}=await import('@/features/pricing/PricingLanding');view=<PricingLanding/>;break;}
    case 'T19':{const {BusinessLanding}=await import('@/features/business/BusinessLanding');view=<BusinessLanding/>;break;}
    case 'T20':{const {CustomerStory}=await import('@/features/customer-story/CustomerStory');view=<CustomerStory/>;break;}
    case 'T21':{const {AboutLanding}=await import('@/features/about/AboutLanding');view=<AboutLanding/>;break;}
    case 'T22':{const {EventLanding}=await import('@/features/event-project/EventLanding');view=<EventLanding/>;break;}
    case 'T23':{
      const {getLegalCatalog}=await import('@/features/legal/catalog.server');
      const {LegalPageFrame}=await import('@/features/legal/LegalPageFrame');
      const {LegalIndexContent}=await import('@/features/legal/LegalPageContent');
      const {getRequestDictionary}=await import('@/i18n/server');
      const locale=await getRequestLocale(),{legal}=await getRequestDictionary(),catalog=getLegalCatalog();
      const documents=catalog.documents.map(doc=>({slug:doc.slug,title:legal.titles[doc.slug]}));
      view=<LegalPageFrame locale={locale} documents={documents}><LegalIndexContent catalog={catalog} locale={locale}/></LegalPageFrame>;break;
    }
    case 'T24':{const {AuthorLanding}=await import('@/features/author/AuthorLanding');const items=source('T24').authorMaterials;view=<AuthorLanding items={items} page={paginateAuthorMaterials(items,searchPage).page}/>;break;}
  }
  const parentPath=page.lessonSlug||page.articleSlug?page.path.slice(0,page.path.lastIndexOf('/')):page.path;
  return <TemplateDataProvider key={page.id} data={{...data,...(page.template==='T24'?{authorPath:parentPath}:{}),__route:{path:page.path,basePath:parentPath}}}>{view}</TemplateDataProvider>;
}
