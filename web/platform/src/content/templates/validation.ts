import {templateDefinitions} from './registry';
import type {ContentPage,Json} from './types';

const reserved=new Set(['app','login','login-v2','invite','api','web','assets','health','ru','en','_next','__dev']);
const recordKeys=new Set(['id','template','path','locale','status','indexing','seo','publishedAt','updatedAt','reviewedAt','relatedIds','content']);
const forbidden=new Set(['__proto__','prototype','constructor']);
export function safeContentPath(path:unknown):path is string{
  return typeof path==='string'&&path.length<=1024&&(path==='/'||/^\/[a-z0-9]+(?:-[a-z0-9]+)*(?:\/[a-z0-9]+(?:-[a-z0-9]+)*)*$/.test(path)&&!reserved.has(path.split('/')[1]));
}
function checkJson(value:unknown,key='',depth=0):asserts value is Json{
  if(depth>40)throw new Error('Content nesting exceeds 40 levels.');
  if(value===null||typeof value==='boolean')return;
  if(typeof value==='number'){if(!Number.isFinite(value))throw new Error('Non-finite content number.');return;}
  if(typeof value==='string'){
    if(value.length>100000)throw new Error('Content field exceeds 100000 characters.');
    // Some existing reference templates prepend their image directory to a poster basename.
    if(key==='poster'&&/^[a-zA-Z0-9][a-zA-Z0-9._-]*\.(?:webp|avif|png|jpe?g|svg)$/.test(value))return;
    if((['href','url','path','src','poster','avatar'].includes(key)||/(?:Path|Url)$/.test(key))&&value){
      if(/[\u0000-\u0020\\]/.test(value)||value.startsWith('//')||!(/^(?:\/(?!\/)|#|https:\/\/|mailto:)/.test(value)))throw new Error(`Unsafe content link: ${key}`);
      if(/^https:\/\//.test(value)){const u=new URL(value);if(u.username||u.password)throw new Error('Credentials in content URL.');}
      if(['src','poster','avatar'].includes(key)&&!value.startsWith('/assets/'))throw new Error('Public media must use /assets/.');
      if(value.startsWith('/en/')||value==='/en')throw new Error('English publication is disabled.');
    }
    if(key==='file'&&(/(?:^|\/)\.\.(?:\/|$)|[\\?#]/.test(value)||value.startsWith('/')))throw new Error('Unsafe media filename.');
    return;
  }
  if(Array.isArray(value)){if(value.length>10000)throw new Error('Content array exceeds 10000 items.');value.forEach(v=>checkJson(v,'',depth+1));return;}
  if(typeof value!=='object'||Object.getPrototypeOf(value)!==Object.prototype)throw new Error('Content must be plain JSON.');
  for(const [k,v]of Object.entries(value)){if(forbidden.has(k))throw new Error('Forbidden content key.');checkJson(v,k,depth+1);}
}

/** Structural contracts from the existing typed template sources. Arrays accept all existing block variants. */
export function validateSource(value:unknown,examples:readonly unknown[],at:string):void{
  const kind=(v:unknown)=>v===null?'null':Array.isArray(v)?'array':typeof v;
  const candidates=examples.filter(v=>kind(v)===kind(value));
  if(!candidates.length)throw new Error(`Incompatible content field: ${at}`);
  if(value===null||typeof value!=='object')return;
  if(Array.isArray(value)){
    const items=candidates.flatMap(v=>v as unknown[]);
    if(items.length)value.forEach((v,i)=>validateSource(v,items,`${at}[${i}]`));
    for(const key of ['id','slug'])if(value.length&&value.every(v=>v&&typeof v==='object'&&typeof v[key as keyof typeof v]==='string')){
      const ids=value.map(v=>(v as Record<string,unknown>)[key]);if(new Set(ids).size!==ids.length)throw new Error(`Duplicate ${key}: ${at}`);
    }
    return;
  }
  let objects=candidates.filter(v=>v&&!Array.isArray(v)) as Record<string,unknown>[];
  const object=value as Record<string,unknown>;
  for(const discriminator of ['type','kind'])if(typeof object[discriminator]==='string'&&objects.some(v=>discriminator in v)){
    objects=objects.filter(v=>v[discriminator]===object[discriminator]);
    if(!objects.length)throw new Error(`Unknown block variant: ${at}.${discriminator}`);
  }
  const keys=new Set(objects.flatMap(v=>Object.keys(v).filter(key=>v[key]!==undefined)));
  for(const key of Object.keys(object))if(!keys.has(key))throw new Error(`Unknown content field: ${at}.${key}`);
  for(const key of keys){
    if(!(key in object)){if(objects.every(v=>v[key]!==undefined))throw new Error(`Missing content field: ${at}.${key}`);continue;}
    validateSource(object[key],objects.filter(v=>v[key]!==undefined).map(v=>v[key]),`${at}.${key}`);
  }
}

export function validateContentPages(input:unknown,defaults:Record<string,Record<string,unknown>>):ContentPage[]{
  checkJson(input);
  if(!Array.isArray(input))throw new Error('Content catalogue must be an array.');
  const ids=new Set<string>(),paths=new Set<string>();
  for(const raw of input){
    if(!raw||Array.isArray(raw)||typeof raw!=='object')throw new Error('Invalid content record.');
    const page=raw as unknown as ContentPage;
    if(Object.keys(raw).some(k=>!recordKeys.has(k)))throw new Error('Unknown content record field.');
    const definition=templateDefinitions.find(t=>t.id===page.template);
    if(!definition)throw new Error('Unknown template.');
    if(typeof page.id!=='string'||!/^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(page.id)||ids.has(page.id))throw new Error('Invalid or duplicate content ID.');ids.add(page.id);
    if(!safeContentPath(page.path)||paths.has(page.path))throw new Error('Invalid or duplicate canonical path.');paths.add(page.path);
    if(page.locale!=='ru')throw new Error('Only Russian content is enabled.');
    if(!['draft','review','published','archived'].includes(page.status)||!['index','noindex'].includes(page.indexing))throw new Error('Invalid publication state.');
    if(!page.seo||Object.keys(page.seo).some(k=>!['title','description'].includes(k))||!page.seo.title?.trim()||!page.seo.description?.trim())throw new Error('SEO title and description are required.');
    for(const name of ['publishedAt','updatedAt','reviewedAt'] as const){const value=page[name];if(value!==undefined&&(!/^\d{4}-\d{2}-\d{2}T.*(?:Z|[+-]\d\d:\d\d)$/.test(value)||!Number.isFinite(Date.parse(value))))throw new Error('Invalid content date.');}
    if(page.status==='published'&&(!page.publishedAt||!page.reviewedAt))throw new Error('Publication requires publishedAt and reviewedAt.');
    if(!Array.isArray(page.relatedIds)||page.relatedIds.some(v=>typeof v!=='string'))throw new Error('Invalid related page IDs.');
    if(!page.content||typeof page.content!=='object'||Array.isArray(page.content))throw new Error('Invalid template content.');
    if((['T01','T18','T23'].includes(page.template))&&page.path!==definition.path)throw new Error('This template has one managed canonical path.');
    if(page.path.startsWith('/legal/'))throw new Error('Legal document paths belong to the managed runtime catalogue.');
    if(definition.managed&&Object.keys(page.content).length)throw new Error('Managed billing/legal sources cannot be overridden.');
    const sourceDefaults=defaults[page.template]??{};
    for(const [key,value]of Object.entries(page.content)){
      if(!Object.prototype.hasOwnProperty.call(sourceDefaults,key))throw new Error('Unknown template source.');
      validateSource(value,[sourceDefaults[key]],key);
    }
    if(page.path!==definition.path&&definition.primary&&!(definition.primary in page.content))throw new Error('New pages require their own primary content.');
    const owned:Record<string,string[]>={T12:['productCampaignWorkflow'],T13:['cinematicCollection'],T14:['flowerExample'],T15:['natureCategory','natureMaterials','initialMaterialFilters'],T16:['gypsophilaMaterial'],T17:['mcpIntegration'],T20:['footballCaseStudy'],T22:['eventProject'],T24:['authorProfile','authorMaterials']};
    if(page.path!==definition.path)for(const key of owned[page.template]??[])if(!(key in page.content))throw new Error(`New pages require their own ${key} source.`);
    if(page.template==='T23'&&page.indexing!=='noindex')throw new Error('Legal publication remains noindex.');
    const data={...sourceDefaults,...page.content} as Record<string,Record<string,unknown>>;
    if(page.template==='T09'){
      const article=data.artPromptsEn as unknown as {sections:{blocks:{type:string;imageIndex?:number}[]}[];lead:{type:string;imageIndex?:number}[];closing:{type:string;imageIndex?:number}[]};
      const media=data.artPromptsReferenceEn as unknown as {illustrations:unknown[]};
      if(!article.sections.length)throw new Error('Article sections cannot be empty.');
      for(const block of [...article.lead,...article.closing,...article.sections.flatMap(s=>s.blocks)])if(block.type==='image'&&(!Number.isInteger(block.imageIndex)||block.imageIndex!<0||block.imageIndex!>=media.illustrations.length))throw new Error('Article image index does not resolve.');
    }
    if(page.template==='T10'){
      const course=data.courseEn as unknown as {modules:{lessons:{slug:string}[]}[]};
      const lessons=course.modules.flatMap(m=>m.lessons.map(l=>l.slug));
      if(!course.modules.length||!lessons.length||new Set(lessons).size!==lessons.length)throw new Error('Course requires unique lessons.');
    }
    if(page.template==='T11'){
      const groups=data.documentationGroups as unknown as {id:string}[],articles=data.documentationArticles as unknown as {group:string}[];
      if(articles.some(a=>!groups.some(g=>g.id===a.group)))throw new Error('Documentation article group does not resolve.');
    }
  }
  for(const page of input as unknown as ContentPage[])for(const id of page.relatedIds)if(!ids.has(id)||id===page.id)throw new Error('Related page ID does not resolve.');
  return input as unknown as ContentPage[];
}
