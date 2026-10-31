import {describe,expect,it} from 'vitest';
import {sitemapDocuments,sitemapIndex} from './sitemap-xml';
describe('large catalogue sitemaps',()=>{
  it('splits sixty thousand URLs without dropping or duplicating a page',()=>{
    const documents=sitemapDocuments(Array.from({length:60000},(_,i)=>({url:`https://epifera.example/ru/blog/article-${i}`})));
    expect(documents).toHaveLength(2);expect(documents.map(d=>(d.match(/<url>/g)??[]).length)).toEqual([45000,15000]);
    expect(documents.join('')).toContain('article-59999</loc>');
  });
  it('also splits by document size and escapes XML',()=>{
    expect(sitemapDocuments([{url:'https://e.test/?a=1&b=2'},{url:'https://e.test/2'}],100,120)).toHaveLength(2);
    expect(sitemapDocuments([{url:'https://e.test/?a=1&b=2'}])[0]).toContain('&amp;b=2');
    expect(sitemapIndex('https://epifera.example',2)).toContain('/sitemap.xml?part=1</loc>');
  });
});
