import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { LocaleProvider } from "@/i18n/LocaleProvider";

const request=vi.hoisted(()=>({locale:"en" as "en"|"ru"}));
vi.mock("server-only",()=>({}));
vi.mock("@/i18n/server",()=>({getRequestLocale:async()=>request.locale}));
import VersionPage,{generateMetadata} from "./page";
import {publicSitemap} from "@/i18n/seo";
import {KlingFamily} from "@/features/model-family/KlingFamily";

beforeEach(()=>{vi.spyOn(HTMLMediaElement.prototype,"play").mockResolvedValue();vi.spyOn(HTMLMediaElement.prototype,"pause").mockImplementation(()=>{});});
afterEach(()=>{cleanup();request.locale="en";vi.restoreAllMocks();});
async function page(){return render(<LocaleProvider locale={request.locale}>{await VersionPage()}</LocaleProvider>);}

describe("Kling 3.0 model-version landing",()=>{
  it.each(["en","ru"]as const)("keeps %s draft/noindex and outside sitemap",async locale=>{
    request.locale=locale;
    expect((await generateMetadata()).robots).toEqual({index:false,follow:false});
    expect(publicSitemap().some(item=>item.url.includes('/ai/kling/kling-3-0'))).toBe(false);
  });
  it("renders Russian content and localized links",async()=>{
    request.locale="ru";const {container}=await page();
    expect((await generateMetadata()).title).toBe("ИИ-видеомодель Kling 3.0 — Epifera");
    expect(container.querySelector('h1')).not.toBeNull();expect(container.querySelector('main')?.textContent).toMatch(/[А-Яа-яЁё]/);expect([...container.querySelectorAll('a[href]')].filter(link=>link.getAttribute('href')?.startsWith('/en/'))).toHaveLength(0);});
  it("renders the complete version layout with one matching shell",async()=>{
    const {container}=await page();
    expect(screen.getAllByRole("heading",{level:1})).toHaveLength(1);
    expect(screen.getAllByRole("banner")).toHaveLength(1);
    expect(screen.getAllByRole("contentinfo")).toHaveLength(1);
    for(const id of ["examples","whats-new","specs","best-for","numbers","questions"])expect(container.querySelector(`#${id}`)).not.toBeNull();
    expect(container.querySelectorAll("#specs tbody tr")).toHaveLength(9);
    expect(container.querySelectorAll("#best-for article")).toHaveLength(3);
    expect(container.querySelectorAll("#numbers [data-research-figure]")).toHaveLength(3);
    expect(container.querySelectorAll("#questions details")).toHaveLength(5);
    expect(container.textContent).not.toMatch(/Trustpilot|ISO 27001|70 \/ 90|450 credits|Every paid plan/);
  });
  it("describes application capabilities without promising unavailable inputs or audio",async()=>{
    await page();const table=screen.getByRole("table",{name:"Kling 3.0 specs"});
    expect(within(table).getByRole("row",{name:/Inputs/})).toHaveTextContent("Text prompt");
    expect(within(table).getByRole("row",{name:/Native audio/})).toHaveTextContent("Not selectable");
    expect(within(table).getByRole("row",{name:/References/})).toHaveTextContent("Not connected");
    expect(within(table).getByRole("row",{name:/Length/})).toHaveTextContent("3–15s");
    expect(within(table).getByRole("row",{name:/Frame rate/})).toHaveTextContent("Not confirmed");
  });
  it("links launches to the existing model without submitting jobs",async()=>{
    const fetch=vi.spyOn(globalThis,"fetch");const {container}=await page();
    for(const link of screen.getAllByRole("link",{name:"Create with Kling 3.0"}))expect(link).toHaveAttribute("href","/ru/app?model=video_kling_v3");
    expect(container.querySelector('a[href*="modelId="]')).toBeNull();
    for(const anchor of container.querySelectorAll<HTMLAnchorElement>('a[href^="#"]'))expect(container.querySelector(anchor.getAttribute("href")!)).not.toBeNull();
    fireEvent.click(screen.getByRole("button",{name:/Pause motion/}));
    expect(fetch).not.toHaveBeenCalled();
  });
  it("opens local gallery media and restores focus after Escape",async()=>{
    const {container}=await page();const trigger=screen.getAllByRole("button",{name:/Open example/})[0];trigger.focus();fireEvent.click(trigger);
    const dialog=screen.getByRole("dialog",{name:"Kling 3.0 reference example"});
    expect(dialog.querySelector("video")).toHaveAttribute("src",expect.stringContaining("/assets/videos/model-version/"));
    expect(dialog).toHaveTextContent("Magnific reference");
    expect(container.querySelector("[data-model-version-page]")).toHaveAttribute("data-paused","true");
    fireEvent.keyDown(window,{key:"Escape"});fireEvent.animationEnd(screen.getByTestId("version-gallery-backdrop"));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();expect(trigger).toHaveFocus();
  });
  it("attributes reference research and connects the family to this version",async()=>{
    const {container,unmount}=await page();
    expect(container.querySelector("#numbers")).toHaveTextContent("not Epifera usage");
    expect(screen.getByRole("link",{name:"Compare Kling versions"})).toHaveAttribute("href","/ru/ai/kling#versions");
    unmount();render(<LocaleProvider locale={request.locale}><KlingFamily/></LocaleProvider>);
    const table=screen.getByRole("table",{name:"Kling versions in Epifera"});
    expect(within(table).getByRole("link",{name:"Kling 3.0"})).toHaveAttribute("href","/ru/ai/kling/kling-3-0");
    expect(within(table).getByRole("link",{name:"View Kling 3.0"})).toHaveAttribute("href","/ru/app?model=video_kling_v3");
  });
});
