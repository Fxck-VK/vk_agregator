import {cleanup,fireEvent,render,screen,within,waitFor} from "@testing-library/react";
import {afterEach,describe,expect,it,vi} from "vitest";
import {LocaleProvider} from "@/i18n/LocaleProvider";
const request=vi.hoisted(()=>({locale:"en" as "en"|"ru"}));
vi.mock("server-only",()=>({}));
vi.mock("@/i18n/server",()=>({getRequestLocale:async()=>request.locale}));
import CollectionPage,{generateMetadata} from "./page";
import {cinematicCollection} from "@/i18n/public/collection-en";
import {publicSitemap} from "@/i18n/seo";
import {readImagePrompt,clearImagePrompt} from "@/features/image-generation/image-prompt-draft";

afterEach(()=>{cleanup();request.locale="en";localStorage.clear();clearImagePrompt(readImagePrompt());vi.restoreAllMocks();});
async function page(){return render(<LocaleProvider locale={request.locale}>{await CollectionPage()}</LocaleProvider>);}
function closePreview(){fireEvent.keyDown(window,{key:"Escape"});fireEvent.animationEnd(screen.getByTestId("collection-preview-backdrop"));}
describe("T13 cinematic collection",()=>{
  it.each(["en","ru"] as const)("keeps %s draft/noindex outside sitemap",async locale=>{
    request.locale=locale;expect((await generateMetadata()).robots).toEqual({index:false,follow:false});
    expect(publicSitemap().some(p=>p.url.includes("/inspiration/cinematic-scenes"))).toBe(false);
  });
  it("renders Russian content and localized links",async()=>{request.locale="ru";const {container}=await page();expect(container.querySelector('h1')).not.toBeNull();expect(container.querySelector('main')?.textContent).toMatch(/[А-Яа-яЁё]/);expect([...container.querySelectorAll('a[href]')].filter(link=>link.getAttribute('href')?.startsWith('/en/'))).toHaveLength(0);});
  it("renders the actual ordered collection with one own shell and local media",async()=>{
    const {container}=await page();expect(screen.getAllByRole("banner")).toHaveLength(1);expect(screen.getAllByRole("contentinfo")).toHaveLength(1);
    expect(screen.getAllByRole("heading",{level:1})).toHaveLength(1);expect(screen.getByRole("heading",{level:1})).toHaveTextContent(cinematicCollection.title);
    const cards=container.querySelectorAll("[data-collection-member]");expect(cards).toHaveLength(48);
    expect([...cards].map(n=>n.getAttribute("data-collection-member"))).toEqual(cinematicCollection.orderedMemberIds);
    expect(container.querySelector('iframe,video,img[src^="http"],a[href*="magnific.com"],input[type="checkbox"]')).toBeNull();
    expect(container.textContent).not.toMatch(/17 Followers|64 images|Premium|Freepik/i);
    expect(screen.queryByRole("button",{name:/theme|language/i})).toBeNull();
  });
  it("searches all members and supports a clear empty state",async()=>{
    const {container}=await page(),input=screen.getByRole("searchbox",{name:"Search this collection"});
    fireEvent.change(input,{target:{value:"forest"}});const found=container.querySelectorAll("[data-collection-member]");expect(found.length).toBeGreaterThan(2);expect(found.length).toBeLessThan(48);
    fireEvent.change(input,{target:{value:"zzzzunmatched"}});expect(screen.getByText("No matching examples")).toBeInTheDocument();expect(container.querySelectorAll("[data-collection-member]")).toHaveLength(0);
    fireEvent.click(screen.getByRole("button",{name:"Clear search"}));expect(container.querySelectorAll("[data-collection-member]")).toHaveLength(48);expect(input).toHaveFocus();
  });
  it("opens previews from the filtered order, supports arrows and returns focus",async()=>{
    const {container}=await page();const root=container.querySelector<HTMLElement>("[data-collection-page]")!;let inert=false;
    Object.defineProperty(root,"inert",{configurable:true,get:()=>inert,set:(value:boolean)=>{inert=value;if(value&&root.contains(document.activeElement))(document.activeElement as HTMLElement).blur();}});
    const trigger=screen.getAllByRole("button",{name:/^View example:/})[0];trigger.focus();fireEvent.click(trigger);
    expect(screen.getByRole("dialog")).toHaveTextContent(cinematicCollection.members[0].alt);expect(root).toHaveProperty("inert",true);
    fireEvent.keyDown(document,{key:"ArrowRight"});expect(screen.getByRole("dialog")).toHaveTextContent(cinematicCollection.members[1].alt);
    closePreview();expect(screen.queryByRole("dialog")).toBeNull();expect(root).toHaveProperty("inert",false);expect(trigger).toHaveFocus();
  });
  it("copies a suggested prompt and hands it off only in memory",async()=>{
    const writeText=vi.fn().mockResolvedValue(undefined);Object.defineProperty(navigator,"clipboard",{value:{writeText},configurable:true});const fetch=vi.spyOn(globalThis,"fetch");
    await page();fireEvent.click(screen.getAllByRole("button",{name:/^View example:/})[0]);const dialog=screen.getByRole("dialog");
    fireEvent.click(within(dialog).getByRole("button",{name:"Copy prompt"}));await waitFor(()=>expect(writeText).toHaveBeenCalledWith(cinematicCollection.members[0].prompt));
    const link=within(dialog).getByRole("link",{name:"Use this prompt"});expect(link).toHaveAttribute("href","/ru/app/image");fireEvent.click(link);
    expect(readImagePrompt()).toBe(cinematicCollection.members[0].prompt);expect(fetch).not.toHaveBeenCalled();expect(localStorage.length).toBe(0);
  });
  it("opens the card Use menu and its preview returns focus to the card action",async()=>{
    await page();const trigger=screen.getByRole("button",{name:`Use ${cinematicCollection.members[0].alt}`});trigger.focus();fireEvent.click(trigger);
    const menu=screen.getByRole("dialog",{name:"Use example"});expect(within(menu).getByRole("link",{name:"Use this prompt"})).toHaveAttribute("href","/ru/app/image");
    fireEvent.click(within(menu).getByRole("button",{name:"View suggested prompt"}));expect(screen.getByRole("dialog",{name:"Example preview"})).toHaveTextContent(cinematicCollection.members[0].alt);
    closePreview();expect(trigger).toHaveFocus();
  });
  it("reports clipboard failure without pretending the prompt was copied",async()=>{
    Object.defineProperty(navigator,"clipboard",{value:{writeText:vi.fn().mockRejectedValue(new Error("blocked"))},configurable:true});await page();fireEvent.click(screen.getAllByRole("button",{name:/^View example:/})[0]);
    fireEvent.click(screen.getByRole("button",{name:"Copy prompt"}));expect(await screen.findByText("Could not copy. Select and copy the prompt below.")).toBeInTheDocument();
  });
  it("saves only a public collection id on this device and can undo it",async()=>{
    await page();fireEvent.click(screen.getByRole("button",{name:"Save collection"}));expect(screen.getByRole("button",{name:"Saved collection"})).toHaveAttribute("aria-pressed","true");
    expect(localStorage.getItem("epifera.saved-collections")).toBe(JSON.stringify([cinematicCollection.id]));
    fireEvent.click(screen.getByRole("button",{name:"Saved collection"}));expect(localStorage.getItem("epifera.saved-collections")).toBe("[]");
  });
  it("restores the device bookmark on a fresh mount",async()=>{
    localStorage.setItem("epifera.saved-collections",JSON.stringify([cinematicCollection.id]));await page();
    expect(screen.getByRole("button",{name:"Saved collection"})).toHaveAttribute("aria-pressed","true");
  });
  it("preserves search order inside the preview",async()=>{
    await page();fireEvent.change(screen.getByRole("searchbox",{name:"Search this collection"}),{target:{value:"forest"}});
    const members=cinematicCollection.members.filter(item=>`${item.alt} ${item.tags.join(" ")} ${item.prompt}`.toLowerCase().includes("forest"));
    fireEvent.click(screen.getAllByRole("button",{name:/^View example:/})[0]);expect(screen.getByRole("dialog")).toHaveTextContent(members[0].alt);
    fireEvent.keyDown(document,{key:"ArrowRight"});expect(screen.getByRole("dialog")).toHaveTextContent(members[1].alt);
    expect(screen.getAllByTestId("collection-preview-thumbnail")).toHaveLength(members.length);
  });
  it("reports unavailable browser storage",async()=>{
    vi.spyOn(Storage.prototype,"setItem").mockImplementation(()=>{throw new Error("blocked");});await page();fireEvent.click(screen.getByRole("button",{name:"Save collection"}));
    expect(screen.getByRole("status")).toHaveTextContent("Could not save on this device.");expect(screen.getByRole("button",{name:"Save collection"})).toHaveAttribute("aria-pressed","false");
  });
  it("shares the collection URL and exposes a fallback when copying fails",async()=>{
    const writeText=vi.fn().mockRejectedValue(new Error("blocked"));Object.defineProperty(navigator,"clipboard",{value:{writeText},configurable:true});await page();fireEvent.click(screen.getByRole("button",{name:"Share collection"}));
    expect(await screen.findByRole("textbox",{name:"Collection link"})).toHaveValue(`${window.location.origin}/en/inspiration/cinematic-scenes`);
  });
  it("opens mobile search and navigation as dismissible dialogs",async()=>{
    await page();fireEvent.click(screen.getByRole("button",{name:"Open collection search"}));const dialog=screen.getByRole("dialog",{name:"Search this collection"});expect(within(dialog).getByRole("searchbox")).toHaveFocus();
    expect(within(dialog).getByRole("button",{name:"Show results"})).toBeInTheDocument();
    fireEvent.keyDown(window,{key:"Escape"});fireEvent.animationEnd(screen.getByTestId("landing-dialog-backdrop"));
    fireEvent.click(screen.getByRole("button",{name:"Open tools"}));expect(screen.getByRole("dialog",{name:"All tools"})).toBeInTheDocument();
    expect(within(screen.getByRole("dialog")).getByRole("link",{name:"Image Generator"})).toHaveAttribute("href","/ru/app/image");
  });
});
