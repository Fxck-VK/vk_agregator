import type {MetadataRoute} from 'next';
const escape=(text:string)=>text.replaceAll('&','&amp;').replaceAll('<','&lt;').replaceAll('>','&gt;').replaceAll('"','&quot;').replaceAll("'",'&apos;');
const header='<?xml version="1.0" encoding="UTF-8"?>';
const open='<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9" xmlns:xhtml="http://www.w3.org/1999/xhtml">';
export function sitemapDocuments(pages:MetadataRoute.Sitemap,maxUrls=45000,maxCharacters=40000000):string[]{
  const documents:string[]=[],rows:string[]=[];let size=open.length;
  for(const page of pages){
    const date=page.lastModified instanceof Date?page.lastModified.toISOString():page.lastModified;
    const row=`<url><loc>${escape(page.url)}</loc>${date?`<lastmod>${escape(date)}</lastmod>`:''}${Object.entries(page.alternates?.languages??{}).map(([language,url])=>`<xhtml:link rel="alternate" hreflang="${escape(language)}" href="${escape(String(url))}"/>`).join('')}</url>`;
    if(rows.length&&(rows.length>=maxUrls||size+row.length>maxCharacters)){documents.push(header+open+rows.join('')+'</urlset>');rows.length=0;size=open.length;}
    rows.push(row);size+=row.length;
  }
  if(rows.length||!documents.length)documents.push(header+open+rows.join('')+'</urlset>');
  return documents;
}
export function sitemapIndex(origin:string,count:number){return header+'<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">'+Array.from({length:count},(_,i)=>`<sitemap><loc>${escape(origin)}/sitemap.xml?part=${i}</loc></sitemap>`).join('')+'</sitemapindex>';}
