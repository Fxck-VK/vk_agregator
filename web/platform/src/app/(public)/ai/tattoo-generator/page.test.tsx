import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { LocaleProvider } from "@/i18n/LocaleProvider";
import { clearImagePrompt, readImagePrompt } from "@/features/image-generation/image-prompt-draft";

const request=vi.hoisted(()=>({locale:"en" as "en"|"ru",push:vi.fn()}));
vi.mock("server-only",()=>({}));
vi.mock("@/i18n/server",()=>({getRequestLocale:async()=>request.locale}));
vi.mock("@/i18n/navigation",()=>({useRouter:()=>({push:request.push}),usePathname:()=>"/ai/tattoo-generator"}));
import TattooPage,{generateMetadata} from "./page";
import {publicSitemap} from "@/i18n/seo";

afterEach(()=>{cleanup();clearImagePrompt(readImagePrompt());request.locale="en";request.push.mockReset();vi.restoreAllMocks();});
async function page(){return render(<LocaleProvider locale={request.locale}>{await TattooPage()}</LocaleProvider>);}
describe("Tattoo task-solution landing",()=>{
  it.each(["en","ru"]as const)("keeps %s draft/noindex and outside sitemap",async locale=>{
    request.locale=locale;expect((await generateMetadata()).robots).toEqual({index:false,follow:false});
    expect(publicSitemap().some(item=>item.url.includes('/ai/tattoo-generator'))).toBe(false);
  });
  it("renders Russian content and localized links",async()=>{request.locale="ru";const {container}=await page();expect(container.querySelector('h1')).not.toBeNull();expect(container.querySelector('main')?.textContent).toMatch(/[А-Яа-яЁё]/);expect([...container.querySelectorAll('a[href]')].filter(link=>link.getAttribute('href')?.startsWith('/en/'))).toHaveLength(0);});
  it("renders the full reference structure with one shell and honest product claims",async()=>{
    const {container}=await page();expect(screen.getAllByRole('heading',{level:1})).toHaveLength(1);expect(screen.getAllByRole('banner')).toHaveLength(1);expect(screen.getAllByRole('contentinfo')).toHaveLength(1);
    for(const id of ['how-it-works','benefits','more-flows','directory','connections','plans','workflow','questions'])expect(container.querySelector(`#${id}`)).not.toBeNull();
    expect(container.querySelectorAll('#more-flows [data-flow-card]')).toHaveLength(7);expect(container.querySelectorAll('#workflow [data-workflow-card]')).toHaveLength(8);expect(container.querySelectorAll('#questions details')).toHaveLength(15);
    expect(container.textContent).not.toMatch(/Unlimited, all year|ISO 27001|Get Premium|RG\/A|Trustpilot/);
    expect(container.querySelector('a[href*="magnific.com/app"]')).toBeNull();
  });
  it("lets presets fill editable fields without preparing a job or prompt handoff",async()=>{
    const fetch=vi.spyOn(globalThis,'fetch');await page();
    fireEvent.click(screen.getByRole('button',{name:'Blackwork'}));expect(screen.getByLabelText('Tattoo style')).toHaveValue('Blackwork');
    fireEvent.click(screen.getByRole('button',{name:'Spine'}));expect(screen.getByLabelText('Where on the body?')).toHaveValue('Spine');
    fireEvent.change(screen.getByLabelText('Tattoo style'),{target:{value:'Fine line with dotwork'}});expect(screen.getByRole('button',{name:'Blackwork'})).toHaveAttribute('aria-pressed','false');
    expect(readImagePrompt()).toBe('');expect(request.push).not.toHaveBeenCalled();expect(fetch).not.toHaveBeenCalled();
  });
  it("requires an idea and stages the complete brief only on explicit submit",async()=>{
    const fetch=vi.spyOn(globalThis,'fetch');const {container}=await page();
    fireEvent.submit(screen.getByRole('form',{name:'Your tattoo inputs'}));expect(screen.getByRole('alert')).toBeInTheDocument();expect(request.push).not.toHaveBeenCalled();expect(readImagePrompt()).toBe('');
    fireEvent.change(screen.getByLabelText('Your tattoo idea'),{target:{value:'  A small crescent and fern  '}});fireEvent.click(screen.getByRole('button',{name:'Fine line'}));fireEvent.click(screen.getByRole('button',{name:'Forearm'}));fireEvent.submit(screen.getByRole('form',{name:'Your tattoo inputs'}));
    expect(readImagePrompt()).toContain('A small crescent and fern');expect(readImagePrompt()).toContain('Fine line');expect(readImagePrompt()).toContain('Forearm');expect(request.push).toHaveBeenCalledExactlyOnceWith('/app/image');expect(fetch).not.toHaveBeenCalled();
    expect(container.querySelector('input[type=file]')).toBeNull();expect(request.push.mock.calls[0][0]).not.toContain('?');
  });
  it("selects all five examples and opens a reversible reference preview",async()=>{
    const {container}=await page();for(let i=1;i<=5;i++){fireEvent.click(screen.getByRole('button',{name:`Show example ${i}`}));expect(screen.getByRole('button',{name:`Show example ${i}`})).toHaveAttribute('aria-pressed','true');}
    const trigger=screen.getByRole('button',{name:'Enlarge selected example'});trigger.focus();fireEvent.click(trigger);const dialog=screen.getByRole('dialog',{name:'Tattoo reference example'});expect(dialog).toHaveTextContent('Magnific reference');expect(dialog.querySelector('img')).not.toBeNull();
    fireEvent.keyDown(window,{key:'Escape'});fireEvent.animationEnd(screen.getByTestId('landing-dialog-backdrop'));expect(screen.queryByRole('dialog')).not.toBeInTheDocument();expect(trigger).toHaveFocus();expect(container.querySelector('[data-tattoo-page]')).toBeInTheDocument();
  });
  it("switches all four instruction steps and their accessible panels",async()=>{
    const {container}=await page();const buttons=within(container.querySelector('#how-it-works')as HTMLElement).getAllByRole('button');
    for(const button of buttons){fireEvent.click(button);expect(button).toHaveAttribute('aria-expanded','true');const id=button.getAttribute('aria-controls')!;expect(container.querySelector(`#${id}`)).not.toHaveAttribute('hidden');}
    buttons[0].focus();fireEvent.keyDown(buttons[0],{key:'End'});expect(buttons[3]).toHaveFocus();
  });
  it("links the planned integration landing and resolves every local anchor",async()=>{
    const {container}=await page();fireEvent.click(screen.getByRole('tab',{name:'Codex'}));expect(screen.getByRole('tabpanel',{name:'Codex'})).toHaveTextContent('Coming soon');
    expect(screen.getByRole('link',{name:'Explore Epifera MCP'})).toHaveAttribute('href','/en/mcp');expect(screen.queryByRole('dialog')).toBeNull();
    expect(container.querySelector('a[href^="cursor:"],a[href^="vscode:"],a[href*="mcp.magnific"]')).toBeNull();
    for(const anchor of container.querySelectorAll<HTMLAnchorElement>('a[href^="#"]'))expect(container.querySelector(anchor.getAttribute('href')!)).not.toBeNull();
  });
});
