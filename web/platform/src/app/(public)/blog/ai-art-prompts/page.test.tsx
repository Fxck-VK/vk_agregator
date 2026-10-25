import {act,cleanup,fireEvent,render,screen,within,waitFor} from "@testing-library/react";
import {afterEach,describe,expect,it,vi} from "vitest";
import {LocaleProvider} from "@/i18n/LocaleProvider";
const request=vi.hoisted(()=>({locale:"en" as "en"|"ru"}));
vi.mock("server-only",()=>({}));
vi.mock("@/i18n/server",()=>({getRequestLocale:async()=>request.locale}));
import ArticlePage,{generateMetadata} from "./page";
import {publicSitemap} from "@/i18n/seo";
import {artPromptsEn} from "@/i18n/public/art-prompts-en";
afterEach(()=>{cleanup();request.locale="en";vi.restoreAllMocks();vi.unstubAllGlobals();window.history.replaceState({},"","/");});
async function page(){return render(<LocaleProvider locale={request.locale}>{await ArticlePage()}</LocaleProvider>);}
describe("Art prompts article",()=>{
  it.each(["en","ru"] as const)("keeps %s draft/noindex outside sitemap",async locale=>{request.locale=locale;expect((await generateMetadata()).robots).toEqual({index:false,follow:false});expect(publicSitemap().some(x=>x.url.includes('/blog/ai-art-prompts'))).toBe(false);});
  it("renders Russian content and localized links",async()=>{request.locale='ru';const {container}=await page();expect(container.querySelector('h1')).not.toBeNull();expect(container.querySelector('main')?.textContent).toMatch(/[А-Яа-яЁё]/);expect([...container.querySelectorAll('a[href]')].filter(link=>link.getAttribute('href')?.startsWith('/en/'))).toHaveLength(0);});
  it("renders one editorial shell with five sections, three illustrations, four examples and three related guides",async()=>{
    const {container}=await page();expect(screen.getAllByRole('heading',{level:1})).toHaveLength(1);expect(screen.getAllByRole('banner')).toHaveLength(1);expect(screen.getByRole('banner')).toHaveAttribute('data-variant','editorial');expect(screen.getAllByRole('contentinfo')).toHaveLength(1);
    expect(container.querySelectorAll('[data-article-section]')).toHaveLength(5);expect(container.querySelectorAll('[data-article-illustration]')).toHaveLength(3);expect(container.querySelectorAll('[data-prompt-example]')).toHaveLength(4);expect(container.querySelectorAll('[data-related-guide]')).toHaveLength(3);
    expect(screen.getByRole('heading',{level:1})).toHaveTextContent('How to write AI art prompts for generating images');expect(container.textContent).not.toMatch(/Sofía López|Published January 2|Hello AI-Addicts|8K resolution ensures/);expect(container.querySelector('iframe,form,textarea,video,audio')).toBeNull();expect(container.querySelector('a[href*="magnific.com"],a[href*="intent/"]')).toBeNull();
  });
  it("keeps every local anchor unique and updates the independent five-entry outline",async()=>{
    const {container}=await page();const ids=[...container.querySelectorAll('[id]')].map(x=>x.id);expect(new Set(ids).size).toBe(ids.length);for(const a of container.querySelectorAll<HTMLAnchorElement>('a[href^="#"]'))expect(container.querySelector(a.getAttribute('href')!)).not.toBeNull();
    const outline=screen.getByRole('navigation',{name:'On this page'});expect(within(outline).getAllByRole('link')).toHaveLength(5);const link=within(outline).getByRole('link',{name:'Tips to write better prompts'});fireEvent.click(link);expect(link).toHaveAttribute('aria-current','location');
  });
  it("keeps article copy safe and dimensions in controls rather than magic prompt wording",()=>{
    expect(artPromptsEn.sections.map(s=>s.id)).toEqual(['what-is-an-ai-prompt','how-do-ai-art-generators-work','how-to-write-an-ai-art-prompt','tips-to-write-better-prompts','ai-art-prompt-examples-to-try']);expect(JSON.stringify(artPromptsEn)).toContain('resolution controls');expect(JSON.stringify(artPromptsEn)).not.toMatch(/<script|dangerouslySetInnerHTML/);
  });
  it("tracks article section visibility independently of comparison ids",async()=>{
    let notify!:(items:Partial<IntersectionObserverEntry>[])=>void;
    vi.stubGlobal('IntersectionObserver',class {constructor(callback:(items:Partial<IntersectionObserverEntry>[])=>void){notify=callback;}observe(){}disconnect(){}});
    const {container}=await page();const target=container.querySelector('#tips-to-write-better-prompts')!;
    act(()=>notify([{target,isIntersecting:true}]));
    expect(within(screen.getByRole('navigation',{name:'On this page'})).getByRole('link',{name:'Tips to write better prompts'})).toHaveAttribute('aria-current','location');
  });
  it("opens listen information without a network request or media generation",async()=>{const fetch=vi.spyOn(globalThis,'fetch');await page();fireEvent.click(screen.getByRole('button',{name:'Listen to this article'}));expect(screen.getByRole('dialog',{name:'Audio reading'})).toHaveTextContent('Coming soon');expect(fetch).not.toHaveBeenCalled();});
  it("shares only this article URL on an explicit click, awaits copying and restores focus",async()=>{
    let resolveCopy!:()=>void;const writeText=vi.fn(()=>new Promise<void>(resolve=>{resolveCopy=resolve;}));vi.stubGlobal('navigator',{clipboard:{writeText},userAgent:'vitest'});window.history.replaceState({},'','/en/blog/ai-art-prompts?draft=1#tips');await page();const trigger=screen.getByRole('button',{name:'Share this article'});trigger.focus();fireEvent.click(trigger);const dialog=screen.getByRole('dialog',{name:'Share this article'});expect(writeText).not.toHaveBeenCalled();fireEvent.click(within(dialog).getByRole('button',{name:'Copy link'}));expect(writeText).toHaveBeenCalledWith(`${window.location.origin}/en/blog/ai-art-prompts`);expect(screen.queryByText('Link copied')).not.toBeInTheDocument();expect(screen.getByRole('button',{name:'Copying…'})).toBeDisabled();resolveCopy();expect(await screen.findByRole('status')).toHaveTextContent('Link copied');fireEvent.keyDown(window,{key:'Escape'});fireEvent.animationEnd(screen.getByTestId('landing-dialog-backdrop'));expect(screen.queryByRole('dialog')).not.toBeInTheDocument();expect(trigger).toHaveFocus();
  });
  it("leaves a selectable link when copying is blocked",async()=>{vi.stubGlobal('navigator',{clipboard:{writeText:vi.fn().mockRejectedValue(new Error('blocked'))},userAgent:'vitest'});await page();fireEvent.click(screen.getByRole('button',{name:'Share this article'}));fireEvent.click(screen.getByRole('button',{name:'Copy link'}));await waitFor(()=>expect(screen.getByRole('status')).toHaveTextContent('Select and copy'));expect(screen.getByRole('textbox',{name:'Page link'})).toHaveAttribute('readonly');});
});
