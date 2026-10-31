import {getPublicOrigin,publicSitemap} from '@/i18n/seo';
import {sitemapDocuments,sitemapIndex} from '@/content/templates/sitemap-xml';
export const dynamic='force-dynamic';
export function GET(request:Request){
  const documents=sitemapDocuments(publicSitemap()),part=new URL(request.url).searchParams.get('part');
  if(part!==null&&(!/^(0|[1-9]\d*)$/.test(part)||Number(part)>=documents.length))return new Response(null,{status:404});
  const body=part!==null?documents[Number(part)]:documents.length===1?documents[0]:sitemapIndex(getPublicOrigin(),documents.length);
  return new Response(body,{headers:{'Content-Type':'application/xml; charset=utf-8','Cache-Control':'public, max-age=300'}});
}
