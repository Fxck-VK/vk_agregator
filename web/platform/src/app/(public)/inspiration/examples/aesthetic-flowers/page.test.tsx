import {cleanup,fireEvent,render,screen,within,waitFor} from "@testing-library/react";
import {afterEach,describe,expect,it,vi} from "vitest";
import {LocaleProvider} from "@/i18n/LocaleProvider";
const request=vi.hoisted(()=>({locale:"en" as "en"|"ru",push:vi.fn()}));
vi.mock("server-only",()=>({}));vi.mock("@/i18n/server",()=>({getRequestLocale:async()=>request.locale}));
vi.mock("@/i18n/navigation",()=>({useRouter:()=>({push:request.push}),usePathname:()=>"/inspiration/examples/aesthetic-flowers"}));
import ExamplePage,{generateMetadata} from "./page";
import {flowerExample} from "@/i18n/public/public-example-en";
import {readImagePrompt,clearImagePrompt} from "@/features/image-generation/image-prompt-draft";
import {publicSitemap} from "@/i18n/seo";

afterEach(()=>{cleanup();request.locale="en";request.push.mockReset();clearImagePrompt(readImagePrompt());localStorage.clear();vi.restoreAllMocks();vi.unstubAllGlobals();});
async function page(){return render(<LocaleProvider locale={request.locale}>{await ExamplePage()}</LocaleProvider>);}
describe("T14 public example",()=>{
  it.each(["en","ru"] as const)("keeps %s draft/noindex outside sitemap",async locale=>{request.locale=locale;expect((await generateMetadata()).robots).toEqual({index:false,follow:false});expect(publicSitemap().some(p=>p.url.includes("/inspiration/examples/"))).toBe(false);});
  it("renders Russian content and localized links",async()=>{request.locale="ru";const {container}=await page();expect(container.querySelector('h1')).not.toBeNull();expect(container.querySelector('main')?.textContent).toMatch(/[А-Яа-яЁё]/);expect([...container.querySelectorAll('a[href]')].filter(link=>link.getAttribute('href')?.startsWith('/en/'))).toHaveLength(0);});
  it("renders one shell, actual local artwork and truthful provenance",async()=>{
    const {container}=await page();expect(screen.getAllByRole("heading",{level:1})).toHaveLength(1);expect(screen.getAllByRole("banner")).toHaveLength(1);expect(screen.getAllByRole("contentinfo")).toHaveLength(1);
    expect(screen.getByRole("heading",{level:1})).toHaveTextContent(flowerExample.title);expect(flowerExample.modelVersionId).toBeNull();expect(flowerExample.origin).toBe("reference");expect(flowerExample.promptPublicationAllowed).toBe(false);
    const image=screen.getByRole("img",{name:flowerExample.media.alt});expect(image).toHaveAttribute("width","606");expect(image).toHaveAttribute("height","1060");
    expect(container.querySelector('iframe,video,img[src^="https:"],input[type=file]')).toBeNull();expect(container.textContent).not.toMatch(/3072|5376|17 Followers|Premium|million subscribers/);
    expect(container.querySelectorAll('[data-example-related]')).toHaveLength(18);expect(screen.queryByRole("button",{name:/theme|language/i})).toBeNull();
  });
  it("shows source information without inventing an original prompt or model",async()=>{
    await page();fireEvent.click(screen.getByRole("button",{name:"Information"}));const dialog=screen.getByRole("dialog",{name:"About this example"});
    expect(dialog).toHaveTextContent("606 × 1060");expect(dialog).toHaveTextContent("Model not provided");expect(dialog).toHaveTextContent("Original prompt not provided");
    expect(within(dialog).getByRole("link",{name:"View source on Magnific"})).toHaveAttribute("href",flowerExample.provenance.url);
  });
  it("offers real downloadable previews with brand filenames",async()=>{
    await page();expect(screen.getByRole("link",{name:"Download preview"})).toHaveAttribute("download",expect.stringMatching(/^epifera-/));
    fireEvent.click(screen.getByRole("button",{name:"Download formats"}));const menu=screen.getByRole("dialog",{name:"Download preview formats"});
    for(const format of flowerExample.downloads){const link=within(menu).getByRole("link",{name:format.label});expect(link).toHaveAttribute("href",format.src);expect(link.getAttribute("download")).toMatch(/^epifera-/);}
  });
  it("opens fullscreen media and restores focus despite native inert blur",async()=>{
    const {container}=await page(),root=container.querySelector<HTMLElement>("[data-public-example]")!;let inert=false;Object.defineProperty(root,"inert",{configurable:true,get:()=>inert,set:(value:boolean)=>{inert=value;if(value&&root.contains(document.activeElement))(document.activeElement as HTMLElement).blur();}});
    const trigger=screen.getByRole("button",{name:"Enlarge image"});trigger.focus();fireEvent.click(trigger);expect(screen.getByRole("dialog",{name:"Image preview"})).toBeInTheDocument();
    fireEvent.keyDown(window,{key:"Escape"});fireEvent.animationEnd(screen.getByTestId("public-example-preview-backdrop"));expect(screen.queryByRole("dialog")).toBeNull();expect(trigger).toHaveFocus();expect(root).toHaveProperty("inert",false);
  });
  it("copies only the editorial suggested prompt",async()=>{
    const writeText=vi.fn().mockResolvedValue(undefined);Object.defineProperty(navigator,"clipboard",{value:{writeText},configurable:true});await page();fireEvent.click(screen.getByRole("button",{name:"View suggested prompt"}));
    fireEvent.click(screen.getByRole("button",{name:"Copy prompt"}));await waitFor(()=>expect(writeText).toHaveBeenCalledWith(flowerExample.suggestedPrompt));expect(screen.getByRole("dialog")).toHaveTextContent("Suggested prompt");
  });
  it("prepares a variation without submitting, persisting or encoding the prompt",async()=>{
    const fetch=vi.spyOn(globalThis,"fetch");await page();const input=screen.getByRole("textbox",{name:"Describe a new variation"});fireEvent.change(input,{target:{value:"A blue background and warm yellow petals"}});
    expect(readImagePrompt()).toBe("");expect(fetch).not.toHaveBeenCalled();fireEvent.submit(screen.getByRole("form",{name:"Create a variation"}));
    expect(readImagePrompt()).toContain(flowerExample.suggestedPrompt);expect(readImagePrompt()).toContain("A blue background and warm yellow petals");expect(request.push).toHaveBeenCalledExactlyOnceWith("/app/image");expect(localStorage.length).toBe(0);expect(fetch).not.toHaveBeenCalled();
  });
  it("presets fill the form, which can be hidden and reopened",async()=>{
    await page();fireEvent.click(screen.getByRole("button",{name:"Warmer colors"}));expect(screen.getByRole("textbox",{name:"Describe a new variation"})).not.toHaveValue("");expect(request.push).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button",{name:"Hide variation prompt"}));expect(screen.queryByRole("form",{name:"Create a variation"})).toBeNull();fireEvent.click(screen.getByRole("button",{name:"Create a variation"}));await waitFor(()=>expect(screen.getByRole("textbox",{name:"Describe a new variation"})).toHaveFocus());
  });
  it("saves only the public example ID on this device",async()=>{
    await page();fireEvent.click(screen.getByRole("button",{name:"Save example"}));expect(screen.getByRole("button",{name:"Saved example"})).toHaveAttribute("aria-pressed","true");expect(localStorage.getItem("epifera.saved-examples")).toBe(JSON.stringify([flowerExample.id]));
    fireEvent.click(screen.getByRole("button",{name:"Saved example"}));expect(localStorage.getItem("epifera.saved-examples")).toBe("[]");
  });
  it("handles denied device storage without a false saved state",async()=>{
    vi.spyOn(Storage.prototype,"setItem").mockImplementation(()=>{throw Error("blocked");});await page();fireEvent.click(screen.getByRole("button",{name:"Save example"}));expect(screen.getByRole("status")).toHaveTextContent("Could not save on this device.");expect(screen.getByRole("button",{name:"Save example"})).toHaveAttribute("aria-pressed","false");
  });
  it("provides a selectable share URL if clipboard access fails",async()=>{
    Object.defineProperty(navigator,"clipboard",{value:{writeText:vi.fn().mockRejectedValue(Error("blocked"))},configurable:true});await page();fireEvent.click(screen.getByRole("button",{name:"Share example"}));
    expect(await screen.findByRole("textbox",{name:"Example link"})).toHaveValue(`${window.location.origin}/en/inspiration/examples/aesthetic-flowers`);
  });
  it("copies a PNG preview from the local asset only",async()=>{
    const write=vi.fn().mockResolvedValue(undefined),items:Record<string,Blob>[]=[];class TestClipboardItem{constructor(data:Record<string,Blob>){items.push(data);}}
    Object.defineProperty(navigator,"clipboard",{value:{write},configurable:true});vi.stubGlobal("ClipboardItem",TestClipboardItem);const fetch=vi.spyOn(globalThis,"fetch").mockResolvedValue({ok:true,blob:async()=>new Blob(["png"],{type:"image/png"})} as Response);
    await page();fireEvent.click(screen.getByRole("button",{name:"Copy image"}));await waitFor(()=>expect(write).toHaveBeenCalledOnce());expect(Object.keys(items[0])).toEqual(["image/png"]);expect(fetch).toHaveBeenCalledExactlyOnceWith(flowerExample.downloads.find(format=>format.mime==="image/png")!.src);vi.unstubAllGlobals();
  });
  it("reports image clipboard failure and keeps the download available",async()=>{
    Object.defineProperty(navigator,"clipboard",{value:{write:vi.fn().mockRejectedValue(Error("blocked"))},configurable:true});await page();fireEvent.click(screen.getByRole("button",{name:"Copy image"}));
    expect(await screen.findByText("Could not copy the image. Download the preview instead.")).toBeInTheDocument();expect(screen.getByRole("link",{name:"Download preview"})).toBeInTheDocument();
  });
  it("filters related cards and opens the chosen image",async()=>{
    const {container}=await page();const before=container.querySelectorAll('[data-example-related="similar"]').length;fireEvent.click(screen.getByRole("button",{name:"Filter by pattern"}));
    const after=container.querySelectorAll('[data-example-related="similar"]');expect(after.length).toBeGreaterThan(0);expect(after.length).toBeLessThan(before);fireEvent.click(within(after[0] as HTMLElement).getByRole("button"));expect(screen.getByRole("dialog",{name:"Image preview"})).toBeInTheDocument();
  });
});
