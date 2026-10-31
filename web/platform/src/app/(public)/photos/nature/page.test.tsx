import {cleanup,fireEvent,render,screen,within,waitFor} from "@testing-library/react";
import {afterEach,describe,expect,it,vi} from "vitest";
import {LocaleProvider} from "@/i18n/LocaleProvider";
const request=vi.hoisted(()=>({locale:"en" as "en"|"ru"}));vi.mock("server-only",()=>({}));vi.mock("@/i18n/server",()=>({getRequestLocale:async()=>request.locale}));
import CategoryPage,{generateMetadata} from "./page";
import {natureMaterials} from "@/i18n/public/material-category-en";
import {publicSitemap} from "@/i18n/seo";
afterEach(()=>{cleanup();localStorage.clear();request.locale="en";vi.restoreAllMocks();});
async function page(){return render(<LocaleProvider locale={request.locale}>{await CategoryPage()}</LocaleProvider>);}
describe("T15 nature material category",()=>{
  it.each(["en","ru"] as const)("keeps %s draft/noindex outside sitemap",async locale=>{request.locale=locale;expect((await generateMetadata()).robots).toEqual({index:false,follow:false});expect(publicSitemap().some(item=>item.url.includes("/photos/nature"))).toBe(false);});
  it("renders Russian content and localized links",async()=>{request.locale="ru";const {container}=await page();expect(container.querySelector('h1')).not.toBeNull();expect(container.querySelector('main')?.textContent).toMatch(/[А-Яа-яЁё]/);expect([...container.querySelectorAll('a[href]')].filter(link=>link.getAttribute('href')?.startsWith('/en/'))).toHaveLength(0);});
  it("renders one reference shell and a real first page without foreign entitlements",async()=>{
    const {container}=await page();expect(screen.getAllByRole('heading',{level:1})).toHaveLength(1);expect(screen.getByRole('heading',{level:1})).toHaveTextContent('Nature Photos');expect(screen.getAllByRole('banner')).toHaveLength(1);expect(screen.getAllByRole('contentinfo')).toHaveLength(1);
    expect(container.querySelectorAll('[data-material-id]')).toHaveLength(24);expect(container.querySelector('iframe,video,img[src^="http"]')).toBeNull();expect(container.textContent).not.toMatch(/Premium|million subscribers|royalty-free|commercial license included/i);expect(screen.queryByRole('button',{name:/theme|language/i})).toBeNull();
  });
  it("links a separate matching botanical card to its detail page without replacing any nature results",async()=>{const{container}=await page();const link=screen.getByRole('link',{name:'View photo details: Dried gypsophila with window shadow on a beige wall'});expect(link).toHaveAttribute('href','/ru/stock/items/dried-gypsophila/gypsophila-01');expect(container.querySelectorAll('[data-material-id]')).toHaveLength(24);});
  it("searches the entire dataset, clears an empty state and resets pagination",async()=>{
    const {container}=await page();fireEvent.click(screen.getByRole('button',{name:'Page 2'}));expect(container.querySelector('[data-material-id]')).toHaveAttribute('data-material-id',natureMaterials[24].id);
    fireEvent.change(screen.getByRole('searchbox',{name:'Search nature photos'}),{target:{value:'zzzzunmatched'}});expect(screen.getByText('No matching materials')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button',{name:'Reset all filters'}));expect(container.querySelectorAll('[data-material-id]')).toHaveLength(24);expect(screen.getByRole('button',{name:'Page 1'})).toHaveAttribute('aria-current','page');
  });
  it("combines an actual source filter with orientation",async()=>{
    const {container}=await page();fireEvent.click(screen.getByRole('button',{name:'AI-generated options'}));const source=screen.getByRole('dialog',{name:'AI-generated'});fireEvent.click(within(source).getByRole('radio',{name:'AI label in source'}));
    fireEvent.click(screen.getByRole('button',{name:'Orientation options'}));const orientation=screen.getByRole('dialog',{name:'Orientation'});fireEvent.click(within(orientation).getByRole('radio',{name:'Landscape'}));
    const ids=[...container.querySelectorAll('[data-material-id]')].map(node=>node.getAttribute('data-material-id'));expect(ids.length).toBeGreaterThan(0);expect(ids.every(id=>natureMaterials.some(item=>item.id===id&&item.sourceAI&&item.orientation==='landscape'))).toBe(true);
  });
  it("changes layout without discarding the current search",async()=>{
    const {container}=await page();fireEvent.change(screen.getByRole('searchbox',{name:'Search nature photos'}),{target:{value:'forest'}});const before=container.querySelectorAll('[data-material-id]').length;fireEvent.click(screen.getByRole('button',{name:'Square'}));expect(container.querySelector('[data-material-grid]')).toHaveAttribute('data-layout','square');expect(screen.getByRole('searchbox')).toHaveValue('forest');expect(container.querySelectorAll('[data-material-id]')).toHaveLength(before);
  });
  it("applies a related topic and deterministic sorting",async()=>{
    const {container}=await page();fireEvent.click(screen.getByRole('button',{name:'Filter topic: Flowers'}));expect(screen.getByRole('button',{name:'Filter topic: Flowers'})).toHaveAttribute('aria-pressed','true');fireEvent.change(screen.getByRole('combobox',{name:'Sort materials'}),{target:{value:'name'}});const ids=[...container.querySelectorAll('[data-material-id]')].map(node=>node.getAttribute('data-material-id')),names=ids.map(id=>natureMaterials.find(item=>item.id===id)!.alt);expect(names).toEqual([...names].sort((a,b)=>a.localeCompare(b,'en')));
  });
  it("offers actual preview downloads and saves only a public ID",async()=>{
    await page();const item=natureMaterials[0],download=screen.getByRole('link',{name:`Download ${item.alt}`});expect(download).toHaveAttribute('href',item.src);expect(download.getAttribute('download')).toMatch(/^epifera-.*\.webp$/);
    fireEvent.click(screen.getByRole('button',{name:`Save ${item.alt}`}));expect(localStorage.getItem('epifera.saved-materials')).toBe(JSON.stringify([item.id]));
  });
  it("opens the selected preview and returns keyboard focus",async()=>{
    await page();const trigger=screen.getByRole('button',{name:`View example: ${natureMaterials[0].alt}`});trigger.focus();fireEvent.click(trigger);expect(screen.getByRole('dialog',{name:'Example preview'})).toHaveTextContent(natureMaterials[0].alt);fireEvent.keyDown(window,{key:'Escape'});fireEvent.animationEnd(screen.getByTestId('collection-preview-backdrop'));expect(trigger).toHaveFocus();
  });
  it("opens mobile filters with the same choices and returns focus",async()=>{
    await page();const trigger=screen.getByRole('button',{name:'Open all filters'});trigger.focus();fireEvent.click(trigger);const dialog=screen.getByRole('dialog',{name:'Filters'});expect(within(dialog).getByRole('combobox',{name:'Orientation'})).toBeInTheDocument();fireEvent.change(within(dialog).getByRole('combobox',{name:'Orientation'}),{target:{value:'portrait'}});fireEvent.click(within(dialog).getByRole('button',{name:'Show results'}));await waitFor(()=>expect(trigger).toHaveFocus());
  });
});
