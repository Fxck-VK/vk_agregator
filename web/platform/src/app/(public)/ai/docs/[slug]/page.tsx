import {contentMetadata,renderContentPage} from '@/content/templates/render';
import {contentPages,expandContentPages} from '@/content/templates/catalog';
type Props={params:Promise<{slug:string}>};
const base="/ai/docs";
export function generateStaticParams(){return expandContentPages(contentPages).filter(p=>p.articleSlug&&p.path.startsWith(base+'/')).map(p=>({slug:p.articleSlug!}));}
export async function generateMetadata({params}:Props){return contentMetadata(base+'/'+(await params).slug);}
export default async function Page({params}:Props){return renderContentPage(base+'/'+(await params).slug);}
