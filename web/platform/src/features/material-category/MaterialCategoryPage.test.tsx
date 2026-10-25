import {cleanup,fireEvent,render,screen} from "@testing-library/react";
import {afterEach,describe,expect,it} from "vitest";
import {LocaleProvider} from "@/i18n/LocaleProvider";
import {natureMaterials} from "@/i18n/public/material-category-en";
import {MaterialCategoryPage} from "./MaterialCategoryPage";
afterEach(()=>{cleanup();localStorage.clear();});
describe("Russian material search",()=>{
  it("shows the complete first page with its localized default query",()=>{
    const {container}=render(<LocaleProvider locale="ru"><MaterialCategoryPage/></LocaleProvider>);
    expect(screen.getByRole("searchbox")).toHaveValue("Природа");
    expect(container.querySelectorAll("[data-material-id]")).toHaveLength(24);
    expect(natureMaterials[0].tags).toContain("nature");
  });
  it("searches Russian captions and topics while keeping canonical topic IDs",()=>{
    const {container}=render(<LocaleProvider locale="ru"><MaterialCategoryPage/></LocaleProvider>);
    fireEvent.change(screen.getByRole("searchbox"),{target:{value:"лавандовое"}});
    expect(container.querySelectorAll("[data-material-id]")).toHaveLength(1);
    expect(container.querySelector("[data-material-id]")).toHaveAttribute("data-material-id",natureMaterials[0].id);
    fireEvent.change(screen.getByRole("searchbox"),{target:{value:"лес"}});
    fireEvent.click(screen.getByRole("button",{name:"Фильтр по теме: Лес"}));
    const ids=[...container.querySelectorAll("[data-material-id]")].map(node=>node.getAttribute("data-material-id"));
    expect(ids.length).toBeGreaterThan(0);
    expect(ids.every(id=>natureMaterials.some(item=>item.id===id&&item.tags.includes("forest")))).toBe(true);
    expect(screen.getByRole("button",{name:"Фильтр по теме: Лес"})).toHaveAttribute("aria-pressed","true");
    expect(screen.getByRole("searchbox")).toHaveValue("лес");
  });
});
