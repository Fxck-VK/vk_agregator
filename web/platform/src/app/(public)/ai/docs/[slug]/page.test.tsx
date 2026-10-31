import {cleanup,fireEvent,render,screen,within,waitFor} from "@testing-library/react";
import {afterEach,describe,expect,it,vi} from "vitest";
import {LocaleProvider} from "@/i18n/LocaleProvider";
const request=vi.hoisted(()=>({locale:"en" as "en"|"ru"}));
vi.mock("server-only",()=>({}));
vi.mock("@/i18n/server",()=>({getRequestLocale:async()=>request.locale,notFound:()=>{throw new Error("DOC_NOT_FOUND");}}));
vi.mock("next/navigation",async importOriginal=>({...await importOriginal<typeof import("next/navigation")>(),notFound:()=>{throw new Error("DOC_NOT_FOUND");}}));
import DocumentationPage,{generateMetadata,generateStaticParams} from "./page";
import {publicSitemap} from "@/i18n/seo";
import {documentationArticles} from "@/i18n/public/documentation-en";

const params=Promise.resolve({slug:"account-management"});
afterEach(()=>{cleanup();request.locale="en";vi.restoreAllMocks();vi.unstubAllGlobals();window.history.replaceState({},"","/");});
async function page(){return render(<LocaleProvider locale={request.locale}>{await DocumentationPage({params})}</LocaleProvider>);}

describe("documentation article",()=>{
  it.each(["en","ru"] as const)("keeps %s draft/noindex and outside sitemap",async locale=>{
    request.locale=locale;expect((await generateMetadata({params})).robots).toEqual({index:false,follow:false});
    expect(publicSitemap().some(p=>p.url.includes("/ai/docs"))).toBe(false);
  });
  it("declares exactly the working article slugs and rejects unknown ones",async()=>{
    expect(generateStaticParams().map(p=>p.slug)).toEqual(documentationArticles.map(a=>a.slug));
    await expect(DocumentationPage({params:Promise.resolve({slug:"missing-article"})})).rejects.toThrow("DOC_NOT_FOUND");
  });
  it("renders Russian content and localized links",async()=>{request.locale="ru";const {container}=await page();expect(container.querySelector('h1')).not.toBeNull();expect(container.querySelector('main')?.textContent).toMatch(/[А-Яа-яЁё]/);expect([...container.querySelectorAll('a[href]')].filter(link=>link.getAttribute('href')?.startsWith('/en/'))).toHaveLength(0);});
  it("uses one help shell, real anchor destinations and truthful account instructions",async()=>{
    const {container}=await page();expect(screen.getAllByRole("banner")).toHaveLength(1);expect(screen.getAllByRole("heading",{level:1})).toHaveLength(1);
    expect(screen.getByRole("heading",{level:1})).toHaveTextContent("Account management");
    const toc=screen.getByRole("navigation",{name:"In this article"});
    for(const link of within(toc).getAllByRole("link")){const id=link.getAttribute("href")!.slice(1);expect(document.getElementById(id)).toBeInTheDocument();}
    expect(within(screen.getByRole("navigation",{name:"Help articles"})).getByRole("link",{name:"Account management"})).toHaveAttribute("aria-current","page");
    expect(container.textContent).toContain("Yandex ID");expect(container.textContent).toContain("VK ID");
    expect(container.textContent).not.toMatch(/Google sign-in|Apple ID|3 devices|Facebook|Magnific|Freepik/);
    expect(screen.getByRole("link",{name:"Next: Email delivery"})).toHaveAttribute("href","/ru/ai/docs/receiving-emails");
    expect(container.querySelector("iframe,video,form")).toBeNull();
  });
  it("finds articles across collapsed groups and restores the catalogue after clearing",async()=>{
    await page();const nav=screen.getByRole("navigation",{name:"Help articles"}),search=within(nav).getByRole("searchbox",{name:"Search help articles"});
    fireEvent.click(within(nav).getByText("Account & troubleshooting",{exact:true}));
    fireEvent.change(search,{target:{value:"SPAM"}});
    expect(within(nav).getByRole("link",{name:"Email delivery"})).toBeVisible();
    expect(within(nav).queryByRole("link",{name:"Quick start"})).toBeNull();
    fireEvent.change(search,{target:{value:"not-a-help-topic-xyz"}});expect(within(nav).getByRole("status")).toHaveTextContent("No articles found");
    fireEvent.change(search,{target:{value:""}});expect(within(nav).getByRole("link",{name:"Account management"})).toBeVisible();
  });
  it("copies a clean public article URL only after explicit copy",async()=>{
    const writeText=vi.fn().mockResolvedValue(undefined);vi.stubGlobal("navigator",{clipboard:{writeText},userAgent:"vitest"});
    window.history.replaceState({},"","/en/ai/docs/account-management?draft=1#sign-in");await page();
    fireEvent.click(screen.getByRole("button",{name:"Share article"}));expect(writeText).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button",{name:"Copy link"}));await waitFor(()=>expect(writeText).toHaveBeenCalledExactlyOnceWith(`${window.location.origin}/en/ai/docs/account-management`));
  });
  it("keeps mobile search in the shared drawer and restores focus after Escape",async()=>{
    const {container}=await page();const trigger=screen.getByRole("button",{name:"Open help articles"});trigger.focus();fireEvent.click(trigger);
    const dialog=screen.getByRole("dialog",{name:"Help articles"});expect(within(dialog).getByRole("button",{name:"Close menu"})).toHaveFocus();
    fireEvent.change(within(dialog).getByRole("searchbox",{name:"Search help articles"}),{target:{value:"backup email"}});
    expect(within(dialog).getByRole("link",{name:"Account management"})).toBeVisible();
    expect(container.querySelector("[data-help-center]")).toHaveProperty("inert",true);
    fireEvent.keyDown(window,{key:"Escape"});fireEvent.animationEnd(screen.getByTestId("documentation-navigation-backdrop"));
    expect(screen.queryByRole("dialog")).toBeNull();expect(trigger).toHaveFocus();
  });
});
