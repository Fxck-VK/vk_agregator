import {contentMetadata,renderContentPage} from '@/content/templates/render';
export async function generateMetadata(){return contentMetadata("/blog/author/epifera");}
export default async function Page({searchParams}:{searchParams:Promise<{page?:string|string[]}>}){return renderContentPage("/blog/author/epifera",(await searchParams).page);}
