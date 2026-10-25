import {cleanup,fireEvent,render,screen,within,waitFor} from '@testing-library/react';
import {afterEach,describe,expect,it,vi} from 'vitest';
import {LocaleProvider} from '@/i18n/LocaleProvider';
const request=vi.hoisted(()=>({locale:'en' as 'en'|'ru'}));
vi.mock('server-only',()=>({}));
vi.mock('@/i18n/server',()=>({getRequestLocale:async()=>request.locale}));
import Page,{generateMetadata} from './page';
import {authorEn as t,authorProfile,authorPath} from '@/i18n/public/author-en';
import {authorMaterials} from '@/features/author/author-catalog';
import {searchAuthorMaterials,paginateAuthorMaterials} from '@/features/author/author-model';
import {publicSitemap} from '@/i18n/seo';
import {ArtPromptsArticle} from '@/features/editorial-article/ArtPromptsArticle';
import {AvatarComparison} from '@/features/comparison/AvatarComparison';
import {brandName} from '@/config/brand';
afterEach(()=>{cleanup();request.locale='en';vi.restoreAllMocks();vi.unstubAllGlobals();});
async function page(value?:string|string[]){return render(<LocaleProvider locale={request.locale}>{await Page({searchParams:Promise.resolve({page:value})})}</LocaleProvider>);}
describe('T24 author archive',()=>{
  it.each(['en','ru'] as const)('keeps %s draft/noindex outside sitemap',async locale=>{request.locale=locale;expect((await generateMetadata()).robots).toEqual({index:false,follow:false});expect(publicSitemap().some(p=>p.url.includes('/blog/author/epifera'))).toBe(false);});
  it("renders Russian content and localized links",async()=>{request.locale='ru';const{container}=await page();expect(container.querySelector('h1')).not.toBeNull();expect(container.querySelector('main')?.textContent).toMatch(/[А-Яа-яЁё]/);expect([...container.querySelectorAll('a[href]')].filter(link=>link.getAttribute('href')?.startsWith('/en/'))).toHaveLength(0);});
  it('owns one editorial shell and twelve linked material cards',async()=>{
    const{container}=await page();expect(screen.getAllByRole('heading',{level:1})).toHaveLength(1);expect(screen.getByRole('heading',{level:1})).toHaveTextContent(brandName);
    expect(screen.getAllByRole('banner')).toHaveLength(1);expect(screen.getByRole('banner')).toHaveAttribute('data-variant','editorial');expect(screen.getAllByRole('contentinfo')).toHaveLength(1);
    expect(container.querySelectorAll('[data-author-material]')).toHaveLength(12);expect(screen.queryByRole('link',{name:'Sign up'})).toBeNull();
  });
  it('uses the editorial brand without inventing a person, biography or dates',async()=>{
    const{container}=await page();expect(authorProfile.name).toBe(brandName);expect(authorProfile.kind).toBe('organization');expect(authorProfile.bio).toBeNull();expect(authorProfile.socials).toEqual([]);
    expect(authorMaterials.every(item=>!item.date||item.date.kind==='draft')).toBe(true);
    expect(container.textContent).not.toMatch(/31 pages|October 11, 2026|million subscribers|Sofía|Creative Signals/);
    expect(container.querySelector('iframe,form,script[type="application/ld+json"],video')).toBeNull();
  });
  it('renders the second and last pages from query params without duplicates',async()=>{
    let view=await page('2');const second=[...view.container.querySelectorAll('[data-author-material]')].map(a=>a.getAttribute('href'));
    expect(second).toEqual(authorMaterials.slice(12,24).map(item=>'/en'+item.href));expect(screen.getByRole('link',{name:'Page 2'})).toHaveAttribute('aria-current','page');
    cleanup();view=await page('999');const last=paginateAuthorMaterials(authorMaterials,999);expect(view.container.querySelectorAll('[data-author-material]')).toHaveLength(last.items.length);
    expect(screen.getByRole('button',{name:t.next})).toBeDisabled();expect(screen.getByRole('button',{name:t.last})).toBeDisabled();
  });
  it('has bounded locale-preserving pagination URLs with a real target anchor',async()=>{
    const{container}=await page();expect(screen.getByRole('button',{name:t.previous})).toBeDisabled();expect(screen.getByRole('button',{name:t.first})).toBeDisabled();
    expect(screen.getByRole('link',{name:t.next})).toHaveAttribute('href','/en'+authorPath+'?page=2#author-materials');expect(container.querySelector('#author-materials')).not.toBeNull();
  });
  it('searches materials from all pages and handles no matches locally',async()=>{
    const fetchData=vi.fn();vi.stubGlobal('fetch',fetchData);await page();fireEvent.click(screen.getByRole('button',{name:t.search}));
    const dialog=screen.getByRole('dialog',{name:t.searchTitle}),input=within(dialog).getByRole('searchbox',{name:t.searchPlaceholder});
    const last=authorMaterials.at(-1)!;fireEvent.change(input,{target:{value:last.title}});expect(within(dialog).getByRole('link',{name:last.category+' '+last.title})).toHaveAttribute('href','/en'+last.href);
    fireEvent.change(input,{target:{value:'<script>unknown material</script>'}});expect(dialog).toHaveTextContent(t.noResults);expect(dialog.querySelector('script')).toBeNull();expect(fetchData).not.toHaveBeenCalled();
    const clear=within(dialog).getByRole('button',{name:t.clear});clear.focus();fireEvent.click(clear);
    expect(input).toHaveValue('');expect(input).toHaveFocus();expect(within(dialog).getAllByRole('link')).toHaveLength(authorMaterials.length);
  });
  it('returns keyboard focus after native inert blurs the search opener',async()=>{
    const{container}=await page(),root=container.querySelector<HTMLElement>('[data-author-page]')!;let inert=false;
    Object.defineProperty(root,'inert',{configurable:true,get:()=>inert,set:(value:boolean)=>{inert=value;if(value&&root.contains(document.activeElement))(document.activeElement as HTMLElement).blur();}});
    const trigger=screen.getByRole('button',{name:t.search});trigger.focus();fireEvent.click(trigger);expect(inert).toBe(true);
    fireEvent.keyDown(window,{key:'Escape'});fireEvent.animationEnd(screen.getByTestId('landing-dialog-backdrop'));await waitFor(()=>expect(trigger).toHaveFocus());expect(inert).toBe(false);
  });
  it('keeps covers same-origin and credits the design reference',async()=>{
    const{container}=await page();expect([...container.querySelectorAll('img')].every(img=>new URL(img.getAttribute('src')!,location.origin).origin===location.origin)).toBe(true);
    fireEvent.click(screen.getByRole('button',{name:t.source.action}));expect(within(screen.getByRole('dialog',{name:t.source.title})).getByRole('link',{name:t.source.link})).toHaveAttribute('href','https://www.magnific.com/blog/author/magnific/');
  });
  it.each([ArtPromptsArticle,AvatarComparison])('connects the existing article byline to the author',Component=>{
    render(<LocaleProvider locale={request.locale}><Component/></LocaleProvider>);expect(screen.getByRole('link',{name:`By ${brandName}`})).toHaveAttribute('href','/en'+authorPath);
  });
  it('does not persist queries or create account/payment/generation writes',async()=>{const fetchData=vi.fn();vi.stubGlobal('fetch',fetchData);await page();fireEvent.click(screen.getByRole('button',{name:t.search}));expect(fetchData).not.toHaveBeenCalled();expect(localStorage.length).toBe(0);expect(location.search).toBe('');});
});
describe('author archive model',()=>{
  it('projects existing material links without duplicates or empty titles',()=>{expect(authorMaterials.length).toBeGreaterThan(24);expect(new Set(authorMaterials.map(item=>item.href)).size).toBe(authorMaterials.length);expect(authorMaterials.every(item=>item.title&&item.href.startsWith('/'))).toBe(true);});
  it('partitions real records into pages exactly once',()=>{const pages=Math.ceil(authorMaterials.length/12),items=Array.from({length:pages},(_,i)=>paginateAuthorMaterials(authorMaterials,i+1).items).flat();expect(items).toEqual(authorMaterials);});
  it.each([undefined,'bad','-1','1.5',['2','3']])('safely normalizes invalid page %s',value=>{expect(paginateAuthorMaterials(authorMaterials,value).page).toBe(1);});
  it('clamps huge pages and supports an empty catalog',()=>{expect(paginateAuthorMaterials(authorMaterials,'999999999999999999').page).toBe(Math.ceil(authorMaterials.length/12));expect(paginateAuthorMaterials([],2)).toMatchObject({page:1,pages:1,items:[]});});
  it('matches case, normalized whitespace and accents across titles/categories',()=>{const items=[{...authorMaterials[0],title:'Café visual brief',category:'Learning'}];expect(searchAuthorMaterials(items,' CAFE  brief ')).toEqual(items);expect(searchAuthorMaterials(items,'learning')).toEqual(items);expect(searchAuthorMaterials(items,'no-match')).toEqual([]);});
});
