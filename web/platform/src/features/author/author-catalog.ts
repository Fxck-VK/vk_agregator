import 'server-only';
import {artPromptsEn} from '@/i18n/public/art-prompts-en';
import {comparisonEn} from '@/i18n/public/comparison-en';
import {courseEn} from '@/i18n/public/course-en';
import {documentationArticles} from '@/i18n/public/documentation-en';
import {productCampaignWorkflow} from '@/i18n/public/workflow-en';
import {authorEn as t,type AuthorMaterial} from '@/i18n/public/author-en';
import {authorReferenceCovers} from '@/i18n/public/author-reference-en';

const coursePath=`/academy/courses/${courseEn.slug}`;
const lessons=courseEn.modules.flatMap(module=>module.lessons);
const material=(id:string,title:string,category:string,href:string,date:AuthorMaterial['date']=null)=>({id,title,category,href,date});
const lesson=(slug:string)=>{
  const found=lessons.find(item=>item.slug===slug);
  if(!found)throw new Error(`Author archive references missing lesson: ${slug}`);
  return material(`lesson-${slug}`,found.title,t.categories.lesson,`${coursePath}/${slug}`);
};
const article=material('art-prompts',artPromptsEn.title,t.categories.article,'/blog/ai-art-prompts',{kind:'draft',iso:'2026-10-25',label:artPromptsEn.draft});
const comparison=material('avatar-comparison',comparisonEn.title,t.categories.comparison,'/compare/ai-avatar-models',{kind:'draft',iso:'2026-10-25',label:comparisonEn.draft});
const first=[
  lesson('describe-motion'),lesson('review-an-image'),article,comparison,
  material('product-campaign',productCampaignWorkflow.title,t.categories.workflow,'/workflows/product-campaign'),
  material('essentials',courseEn.title,t.categories.course,coursePath),
  lesson('choose-a-model'),lesson('write-a-visual-brief'),lesson('work-with-image-references'),
  lesson('set-framing-and-size'),lesson('review-video-settings'),lesson('prepare-a-music-brief'),
];
const used=new Set(first.map(item=>item.id));
const remaining=[
  ...lessons.filter(item=>!used.has(`lesson-${item.slug}`)).map(item=>lesson(item.slug)),
  ...documentationArticles.map(item=>material(`doc-${item.slug}`,item.title,t.categories.documentation,`/ai/docs/${item.slug}`)),
];
export const authorMaterials:readonly AuthorMaterial[]=[...first,...remaining].map((item,index)=>({...item,cover:authorReferenceCovers[index%authorReferenceCovers.length]}));
