import {cleanup,render,screen} from "@testing-library/react";
import {afterEach,describe,expect,it,vi} from "vitest";
import {LocaleProvider} from "@/i18n/LocaleProvider";
const request=vi.hoisted(()=>({locale:"en" as "en"|"ru"}));
vi.mock("server-only",()=>({}));vi.mock("@/i18n/server",()=>({getRequestLocale:async()=>request.locale}));
import HelpCenterPage,{generateMetadata} from "./page";
import {documentationArticles} from "@/i18n/public/documentation-en";
afterEach(()=>{cleanup();request.locale="en";});
describe("help directory",()=>{
  it("links every actual article from the directory with one shell",async()=>{
    render(<LocaleProvider locale={request.locale}>{await HelpCenterPage()}</LocaleProvider>);
    expect(screen.getAllByRole("banner")).toHaveLength(1);
    expect(screen.getByRole("heading",{level:1})).toHaveTextContent("Help Center");
    for(const article of documentationArticles)expect(screen.getByRole("link",{name:`${article.title} ${article.description}`})).toHaveAttribute("href",`/en/ai/docs/${article.slug}`);
  });
  it.each(["en","ru"] as const)("keeps %s directory draft/noindex",async locale=>{request.locale=locale;expect((await generateMetadata()).robots).toEqual({index:false,follow:false});});
  it("renders Russian content and localized links",async()=>{request.locale="ru";const {container}=render(await HelpCenterPage());expect(container.querySelector('h1')).not.toBeNull();expect(container.querySelector('main')?.textContent).toMatch(/[А-Яа-яЁё]/);expect([...container.querySelectorAll('a[href]')].filter(link=>link.getAttribute('href')?.startsWith('/en/'))).toHaveLength(0);});
});
