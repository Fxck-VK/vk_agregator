import {cleanup,render,screen} from "@testing-library/react";
import {afterEach,describe,expect,it,vi} from "vitest";
import {LocaleProvider} from "@/i18n/LocaleProvider";
const request=vi.hoisted(()=>({locale:"en" as "en"|"ru"}));
vi.mock("server-only",()=>({}));vi.mock("@/i18n/server",()=>({getRequestLocale:async()=>request.locale,notFound:()=>{throw new Error('NOT_FOUND');}}));
vi.mock('next/navigation',async original=>({...await original<typeof import('next/navigation')>(),notFound:()=>{throw new Error('NOT_FOUND');}}));
import LessonPage,{generateMetadata} from "./page";
import {courseLessons} from "@/features/academy/course-model";
import {publications} from "./publication";
afterEach(()=>{cleanup();request.locale='en';});
const props=(lessonSlug:string)=>({params:Promise.resolve({lessonSlug})});
describe('Academy lessons',()=>{
  it('declares all real lesson paths as draft/noindex',()=>{expect(publications).toHaveLength(14);for(const p of publications){expect(p.status).toBe('draft');expect(p.indexing).toBe('noindex');expect(courseLessons.some(l=>p.paths.en?.endsWith(`/${l.slug}`))).toBe(true);}});
  it.each(['en','ru'] as const)('keeps %s noindex',async locale=>{request.locale=locale;expect((await generateMetadata(props(courseLessons[0].slug))).robots).toEqual({index:false,follow:false});});
  it("renders Russian content and localized links",async()=>{request.locale='ru';const {container}=render(<LocaleProvider locale="ru">{await LessonPage(props(courseLessons[0].slug))}</LocaleProvider>);expect(container.querySelector('h1')).not.toBeNull();expect(container.querySelector('main')?.textContent).toMatch(/[А-Яа-яЁё]/);expect([...container.querySelectorAll('a[href]')].filter(link=>link.getAttribute('href')?.startsWith('/en/'))).toHaveLength(0);});
  it('renders a lesson with a real exercise, result, current tree item and bounded previous/next',async()=>{const current=courseLessons[1];render(<LocaleProvider locale={request.locale}>{await LessonPage(props(current.slug))}</LocaleProvider>);expect(screen.getByRole('heading',{level:1})).toHaveTextContent(current.title);expect(screen.getByRole('heading',{name:'Try it yourself'})).toBeInTheDocument();expect(screen.getByRole('heading',{name:'What you should have'})).toBeInTheDocument();expect(screen.getByRole('link',{name:'Previous lesson'})).toHaveAttribute('href',`/en/academy/courses/epifera-essentials/${courseLessons[0].slug}`);expect(screen.getByRole('link',{name:'Next lesson'})).toHaveAttribute('href',`/en/academy/courses/epifera-essentials/${courseLessons[2].slug}`);expect(screen.getByRole('link',{name:current.title})).toHaveAttribute('aria-current','page');});
  it('returns to the course after the final lesson without fake completion',async()=>{render(<LocaleProvider locale={request.locale}>{await LessonPage(props(courseLessons.at(-1)!.slug))}</LocaleProvider>);expect(screen.queryByRole('link',{name:'Next lesson'})).toBeNull();expect(screen.getAllByRole('link',{name:'Back to course'}).length).toBeGreaterThan(0);expect(screen.queryByText(/completed|certificate/i)).toBeNull();});
  it('rejects an unknown lesson',async()=>{await expect(LessonPage(props('unknown-lesson'))).rejects.toThrow('NOT_FOUND');});
});
