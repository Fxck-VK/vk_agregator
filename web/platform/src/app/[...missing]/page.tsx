import {resolveContentPage} from '@/content/templates/catalog';
import {contentMetadata,renderContentRecord} from '@/content/templates/render';
import {MissingContentPage} from '@/content/templates/MissingContentPage';
type Props={params:Promise<{missing:string[]}>;searchParams:Promise<{page?:string|string[]}>};
export async function generateMetadata({params}:Props){
  const path='/'+(await params).missing.join('/');
  return resolveContentPage(path,{preview:process.env.NODE_ENV==='development'})?contentMetadata(path):{robots:{index:false,follow:false}};
}
export default async function MissingPage({params,searchParams}:Props){
  const page=resolveContentPage('/'+(await params).missing.join('/'),{preview:process.env.NODE_ENV==='development'});
  return page?renderContentRecord(page,(await searchParams).page):<MissingContentPage/>;
}
