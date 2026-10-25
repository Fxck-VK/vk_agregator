import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { LocaleProvider } from "@/i18n/LocaleProvider";
const request = vi.hoisted(() => ({ locale: "en" as "en" | "ru" }));
vi.mock("server-only", () => ({}));
vi.mock("@/i18n/server", () => ({ getRequestLocale: async () => request.locale }));
import MarketingPage, { generateMetadata } from "./page";
import { publicSitemap } from "@/i18n/seo";

beforeEach(() => { vi.spyOn(HTMLMediaElement.prototype,"pause").mockImplementation(()=>{}); vi.spyOn(HTMLMediaElement.prototype,"play").mockResolvedValue(); });
afterEach(() => { cleanup(); request.locale="en"; vi.restoreAllMocks(); vi.unstubAllGlobals(); });
async function page() { return render(<LocaleProvider locale={request.locale}>{await MarketingPage()}</LocaleProvider>); }
describe("Marketing audience landing", () => {
  it.each(["en","ru"] as const)("keeps %s draft/noindex outside sitemap", async locale => { request.locale=locale; expect((await generateMetadata()).robots).toEqual({index:false,follow:false}); expect(publicSitemap().some(item=>item.url.includes('/ai/marketing'))).toBe(false); });
  it("renders Russian content and localized links", async () => { request.locale="ru"; const {container}=await page(); expect(container.querySelector('h1')).not.toBeNull();expect(container.querySelector('main')?.textContent).toMatch(/[А-Яа-яЁё]/);expect([...container.querySelectorAll('a[href]')].filter(link=>link.getAttribute('href')?.startsWith('/en/'))).toHaveLength(0);});
  it("renders one matching shell and the complete audience structure",async()=>{
    const {container}=await page();expect(screen.getAllByRole('heading',{level:1})).toHaveLength(1);expect(screen.getAllByRole('banner')).toHaveLength(1);expect(screen.getAllByRole('contentinfo')).toHaveLength(1);
    for(const id of ['workflows','marketing-needs','marketing-guides','plans','feedback','marketing-tools'])expect(container.querySelector(`#${id}`)).not.toBeNull();
    expect(container.querySelectorAll('[data-marketing-example]')).toHaveLength(3);expect(container.querySelectorAll('[data-marketing-guide]')).toHaveLength(11);expect(container.querySelectorAll('[data-marketing-tool]')).toHaveLength(16);
    expect(container.textContent).not.toMatch(/600,000|Ogilvy|Cocacola|HelloFresh|Get Premium|No model training|Unlimited, all year|Save 25%|10k/i);
    expect(container.querySelector('iframe,a[href*="magnific.com"],a[href*="youtube.com"]')).toBeNull();
  });
  it("filters each example category and restores the complete list without a request",async()=>{
    const fetch=vi.spyOn(globalThis,'fetch');const {container}=await page();const filters=screen.getByRole('group',{name:'Workflow categories'});
    for(const [category,title]of [['Branding','Restyle illustration'],['Advertising','Outdoor placement mockup'],['Fashion','Apply design to t-shirt mockup']]){
      fireEvent.click(within(filters).getByRole('button',{name:category}));const cards=container.querySelectorAll('[data-marketing-example]');expect(cards).toHaveLength(1);expect(cards[0]).toHaveTextContent(title);expect(within(filters).getByRole('button',{name:category})).toHaveAttribute('aria-pressed','true');
    }
    fireEvent.click(within(filters).getByRole('button',{name:'All'}));expect(container.querySelectorAll('[data-marketing-example]')).toHaveLength(3);expect(fetch).not.toHaveBeenCalled();
  });
  it("switches all four task panels with keyboard focus and relationships",async()=>{
    await page();for(const label of ['Campaigns','Social content','Branding','Audiences']){fireEvent.click(screen.getByRole('tab',{name:label}));expect(screen.getByRole('tabpanel',{name:label})).toBeVisible();}
    const first=screen.getByRole('tab',{name:'Campaigns'});first.focus();fireEvent.keyDown(first,{key:'End'});expect(screen.getByRole('tab',{name:'Audiences'})).toHaveFocus();expect(screen.getByRole('tabpanel',{name:'Audiences'})).toBeVisible();
  });
  it("opens a local guide and closes it with focus returned to the trigger",async()=>{
    await page();const trigger=screen.getByRole('button',{name:'Apply different styles to your images'});trigger.focus();fireEvent.click(trigger);const dialog=screen.getByRole('dialog',{name:'Apply different styles to your images'});expect(dialog.querySelectorAll('li')).toHaveLength(3);expect(dialog).toHaveTextContent('Magnific reference artwork');expect(within(dialog).getByRole('link',{name:'Open image generator'})).toHaveAttribute('href','/en/app/image');expect(dialog.querySelector('video,iframe')).toBeNull();
    fireEvent.keyDown(window,{key:'Escape'});fireEvent.animationEnd(screen.getByTestId('landing-dialog-backdrop'));expect(screen.queryByRole('dialog')).not.toBeInTheDocument();expect(trigger).toHaveFocus();
  });
  it("shows unsupported standalone tools as planned without fake launch links",async()=>{
    const {container}=await page();fireEvent.click(screen.getByRole('button',{name:/^Spaces /}));const dialog=screen.getByRole('dialog',{name:'Spaces'});expect(dialog).toHaveTextContent('Coming soon');expect(dialog.querySelector('a[href*="spaces"]')).toBeNull();expect(container.querySelector('a[href^="cursor:"],a[href*="partners"]')).toBeNull();
  });
  it("keeps the local hero muted and offers a reversible pause control",async()=>{
    const {container}=await page();const video=container.querySelector('video')!;expect(video.muted).toBe(true);expect(video.loop).toBe(true);expect(container.querySelector('[data-ui="video-poster"]')).toHaveAttribute('src','/assets/images/marketing-audience/b18703cb43f990b71e33.webp');
    fireEvent.click(screen.getByRole('button',{name:'Pause motion'}));expect(screen.getByRole('button',{name:'Resume motion'})).toHaveAttribute('aria-pressed','true');
  });
  it("uses existing workspace destinations and resolves all local anchors",async()=>{const {container}=await page();for(const a of container.querySelectorAll<HTMLAnchorElement>('a[href^="#"]'))expect(container.querySelector(a.getAttribute('href')!)).not.toBeNull();expect(screen.getByRole('link',{name:'Share your feedback'})).toHaveAttribute('href','mailto:support@epifera.com');});
  it("keeps the hero static for reduced motion even when the media becomes visible and ready",async()=>{
    vi.stubGlobal('matchMedia',vi.fn().mockReturnValue({matches:true,addEventListener:vi.fn(),removeEventListener:vi.fn()}));
    vi.stubGlobal('IntersectionObserver',class{constructor(private readonly callback:(entries:{isIntersecting:boolean}[])=>void){}observe(){this.callback([{isIntersecting:true}]);}disconnect(){}});
    const {container}=await page();fireEvent.canPlay(container.querySelector('video')!);expect(HTMLMediaElement.prototype.play).not.toHaveBeenCalled();expect(HTMLMediaElement.prototype.pause).toHaveBeenCalled();
  });
});
