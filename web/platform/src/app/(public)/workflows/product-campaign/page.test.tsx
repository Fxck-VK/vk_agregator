import {cleanup,fireEvent,render,screen,within} from "@testing-library/react";
import {afterEach,describe,expect,it,vi} from "vitest";
import {LocaleProvider} from "@/i18n/LocaleProvider";
const request=vi.hoisted(()=>({locale:"en" as "en"|"ru"}));
vi.mock("server-only",()=>({}));vi.mock("@/i18n/server",()=>({getRequestLocale:async()=>request.locale}));
import WorkflowPage,{generateMetadata} from "./page";
import {publicSitemap} from "@/i18n/seo";
import {productCampaignWorkflow,workflowEn} from "@/i18n/public/workflow-en";
afterEach(()=>{cleanup();request.locale="en";vi.restoreAllMocks();});
async function page(){return render(<LocaleProvider locale={request.locale}>{await WorkflowPage()}</LocaleProvider>);}
describe("workflow reference template",()=>{
  it.each(["en","ru"] as const)("keeps %s draft/noindex outside sitemap",async locale=>{request.locale=locale;expect((await generateMetadata()).robots).toEqual({index:false,follow:false});expect(publicSitemap().some(p=>p.url.includes('/workflows/'))).toBe(false);});
  it("renders Russian content and localized links",async()=>{request.locale="ru";const {container}=await page();expect(container.querySelector('h1')).not.toBeNull();expect(container.querySelector('main')?.textContent).toMatch(/[А-Яа-яЁё]/);expect([...container.querySelectorAll('a[href]')].filter(link=>link.getAttribute('href')?.startsWith('/en/'))).toHaveLength(0);});
  it("renders a complete guide with one shell and local media",async()=>{
    const {container}=await page();expect(screen.getAllByRole('banner')).toHaveLength(1);expect(screen.getAllByRole('contentinfo')).toHaveLength(1);expect(screen.getAllByRole('heading',{level:1})).toHaveLength(1);
    expect(screen.getByRole('heading',{level:1})).toHaveTextContent(productCampaignWorkflow.title);
    expect(productCampaignWorkflow.executionMode).toBe('guide');expect(container.querySelectorAll('[data-workflow-scenario]')).toHaveLength(10);
    for(const link of container.querySelectorAll<HTMLAnchorElement>('a[href^="#"]'))expect(document.getElementById(link.hash.slice(1))).not.toBeNull();
    expect(container.querySelector('iframe,video,form,a[href*="magnific.com"],img[src^="https:"]')).toBeNull();
    expect(container.textContent).not.toMatch(/600,000|Gizem|Ogilvy|Freepik|create up to 3 Spaces|simultaneous editing/i);
  });
  it("opens a real bounded walkthrough without a network request or submission",async()=>{
    const fetch=vi.spyOn(globalThis,'fetch');await page();const trigger=screen.getByRole('button',{name:'Explore the workflow'});trigger.focus();fireEvent.click(trigger);
    const dialog=screen.getByRole('dialog',{name:productCampaignWorkflow.title});expect(dialog).toHaveTextContent('Guided workflow');expect(dialog).toHaveTextContent(productCampaignWorkflow.inputs[0]);
    expect(within(dialog).getByRole('button',{name:'Previous step'})).toBeDisabled();
    for(let i=1;i<productCampaignWorkflow.steps.length;i++)fireEvent.click(within(dialog).getByRole('button',{name:'Next step'}));
    expect(within(dialog).getByRole('button',{name:'Next step'})).toBeDisabled();expect(dialog).toHaveTextContent(productCampaignWorkflow.steps.at(-1)!.title);
    expect(within(dialog).getByRole('link',{name:'Continue in workspace'})).toHaveAttribute('href','/ru/app/files');expect(fetch).not.toHaveBeenCalled();
    fireEvent.keyDown(window,{key:'Escape'});fireEvent.animationEnd(screen.getByTestId('landing-dialog-backdrop'));expect(screen.queryByRole('dialog')).toBeNull();expect(trigger).toHaveFocus();
  });
  it("supports keyboard step selection and its matching tool destination",async()=>{
    await page();fireEvent.click(screen.getByRole('button',{name:'Explore the workflow'}));const dialog=screen.getByRole('dialog'),tabs=within(dialog).getAllByRole('tab');
    tabs[0].focus();fireEvent.keyDown(tabs[0],{key:'End'});expect(tabs.at(-1)).toHaveFocus();expect(tabs.at(-1)).toHaveAttribute('aria-selected','true');
    fireEvent.keyDown(tabs.at(-1)!,{key:'Home'});expect(tabs[0]).toHaveFocus();expect(within(dialog).getByRole('tabpanel')).toHaveTextContent(productCampaignWorkflow.steps[0].title);
  });
  it("restores the opener when native inert blurs it before dialog effects run",async()=>{
    const {container}=await page();const root=container.querySelector<HTMLElement>('[data-workflow-page]')!;let inert=false;
    Object.defineProperty(root,'inert',{configurable:true,get:()=>inert,set:(value:boolean)=>{inert=value;if(value&&root.contains(document.activeElement))(document.activeElement as HTMLElement).blur();}});
    const trigger=screen.getByRole('button',{name:'Explore the workflow'});trigger.focus();fireEvent.click(trigger);
    expect(screen.getByRole('button',{name:'Close'})).toHaveFocus();fireEvent.keyDown(window,{key:'Escape'});fireEvent.animationEnd(screen.getByTestId('landing-dialog-backdrop'));
    expect(root).toHaveProperty('inert',false);expect(trigger).toHaveFocus();
  });
  it.each(Array.from({length:10},(_,i)=>i))("opens scenario %i with its own brief and steps",async i=>{
    await page();const scenario=workflowEn.scenarios[i];fireEvent.click(screen.getByRole('button',{name:scenario.title}));const dialog=screen.getByRole('dialog',{name:scenario.title});
    expect(dialog).toHaveTextContent(scenario.inputs[0]);expect(dialog).toHaveTextContent(scenario.steps[0].title);expect(within(dialog).getAllByRole('tab')).toHaveLength(scenario.steps.length);
    expect(within(dialog).getByRole('link',{name:'Continue in workspace'}).getAttribute('href')).toMatch(/^\/ru\/app(?:\/|$)/);
  });
  it("onboarding opens the genuine guide and FAQ answers remain readable",async()=>{
    await page();fireEvent.click(screen.getByRole('button',{name:'Open workflow walkthrough'}));expect(screen.getByRole('dialog')).toHaveTextContent(productCampaignWorkflow.outputs[0]);
    fireEvent.keyDown(window,{key:'Escape'});fireEvent.animationEnd(screen.getByTestId('landing-dialog-backdrop'));
    const question=screen.getByText(workflowEn.faq[0].question,{exact:true});fireEvent.click(question);expect(question.closest('details')).toHaveAttribute('open');
  });
});
