import {contentMetadata,renderContentPage} from '@/content/templates/render';
type Props={params:Promise<{lessonSlug:string}>};
const base="/academy/courses/epifera-essentials";
export async function generateMetadata({params}:Props){return contentMetadata(base+'/'+(await params).lessonSlug);}
export default async function Page({params}:Props){return renderContentPage(base+'/'+(await params).lessonSlug);}
