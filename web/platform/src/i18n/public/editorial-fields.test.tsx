import {cleanup,fireEvent,render,screen} from "@testing-library/react";
import {afterEach,describe,expect,it,vi} from "vitest";
import {LocaleProvider} from "@/i18n/LocaleProvider";
import {translatePublicText,localizePublicData} from "./public-copy";
import {StoryCredits,StoryFacts} from "@/features/customer-story/StorySections";
import {MaterialDetailPage} from "@/features/material-detail/MaterialDetailPage";
import {gypsophilaMaterial,materialDetailEn} from "./material-detail-en";
import {materialFilters} from "./material-category-en";
import {AboutFocus} from "@/features/about/AboutSections";

vi.mock("@/i18n/navigation",()=>({useRouter:()=>({push:vi.fn()}),usePathname:()=>"/ru"}));
afterEach(()=>{cleanup();localStorage.clear();});

describe("Russian labels in protected editorial fields",()=>{
  it("translates the about-page focus section stored under the people key",()=>{
    render(<LocaleProvider locale="ru"><AboutFocus/></LocaleProvider>);
    expect(screen.queryByText("From choosing a model to reviewing a result, these are the parts of the workspace we focus on.")).not.toBeInTheDocument();
    expect(screen.getByRole("link",{name:/Найти отправную точку/})).toHaveAttribute("href","/ru/ai");
  });
  it("translates credit roles and fact values without translating names or identifiers",()=>{
    render(<LocaleProvider locale="ru"><StoryCredits/><StoryFacts onSource={()=>{}}/></LocaleProvider>);
    expect(screen.queryByText("Creative production")).not.toBeInTheDocument();
    expect(screen.queryByText("CTV · Digital · Social")).not.toBeInTheDocument();
    expect(screen.getByText("CTV · Цифровые каналы · Соцсети")).toBeInTheDocument();
    expect(screen.getByText("Цветокоррекция")).toBeInTheDocument();
    expect(screen.getAllByText("Lenovo").length).toBeGreaterThan(0);
    expect(screen.getByText("FIFA World Cup 2026")).toBeInTheDocument();
  });
  it("uses a color filter label separately from the colorist credit",()=>{
    const filter=localizePublicData(materialFilters,"ru").find(item=>item.key==="color");
    expect(filter?.label).toBe("Основной цвет");
    expect(translatePublicText("Color","ru")).toBe("Цветокоррекция");
  });
  it("translates download and license labels while preserving file and license data",()=>{
    render(<LocaleProvider locale="ru"><MaterialDetailPage material={gypsophilaMaterial}/></LocaleProvider>);
    expect(screen.getByRole("button",{name:"Форматы скачивания"})).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button",{name:translatePublicText(materialDetailEn.license,"ru")}));
    expect(screen.getByRole("heading",{name:"Использование и лицензия"})).toBeInTheDocument();
    expect(localizePublicData(gypsophilaMaterial,"ru").licenseRef).toBe(gypsophilaMaterial.licenseRef);
    expect(gypsophilaMaterial.downloads[0].mime).toBe("image/webp");
  });
  it.each(["Review connection status","Planned connection","Available tools","Guided workflows","Planned team capacity","Existing workspace","Project concept"])("translates the editorial status %s",source=>{
    expect(translatePublicText(source,"ru")).not.toBe(source);
    expect(localizePublicData({status:source},"ru").status).toBe(translatePublicText(source,"ru"));
    expect(localizePublicData({status:"reference"},"ru").status).toBe("reference");
  });
});
