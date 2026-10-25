import {cleanup,fireEvent,render,screen,within} from "@testing-library/react";
import {afterEach,describe,expect,it,vi} from "vitest";
import {LocaleProvider} from "@/i18n/LocaleProvider";
import {localizePublicData} from "@/i18n/public/public-copy";
import {cinematicCollection} from "@/i18n/public/collection-en";
import {flowerExample,publicExampleEn} from "@/i18n/public/public-example-en";
import {gypsophilaMaterial} from "@/i18n/public/material-detail-en";
import {productCampaignWorkflow,workflowEn} from "@/i18n/public/workflow-en";
import {ExampleCollection} from "./ExampleCollection";
import {PublicExamplePage} from "@/features/public-example/PublicExamplePage";
import {MaterialDetailPage} from "@/features/material-detail/MaterialDetailPage";
import {WorkflowLanding} from "@/features/workflow/WorkflowLanding";
vi.mock("@/i18n/navigation",()=>({useRouter:()=>({push:vi.fn()}),usePathname:()=>"/inspiration"}));
afterEach(()=>{cleanup();localStorage.clear();vi.restoreAllMocks();});
describe("Russian editorial record props",()=>{
  it.each([
    ["collection",<ExampleCollection key="collection" collection={cinematicCollection}/>,cinematicCollection],
    ["public example",<PublicExamplePage key="example" example={flowerExample}/>,flowerExample],
    ["material detail",<MaterialDetailPage key="material" material={gypsophilaMaterial}/>,gypsophilaMaterial],
    ["workflow",<WorkflowLanding key="workflow" workflow={productCampaignWorkflow}/>,productCampaignWorkflow],
  ] as const)("localizes the incoming %s record without mutating its source",(_name,component,record)=>{
    const expected=localizePublicData(record,"ru").title;
    expect(expected).not.toBe(record.title);
    render(<LocaleProvider locale="ru">{component}</LocaleProvider>);
    expect(screen.getByRole("heading",{level:1})).toHaveTextContent(expected);
    expect(record.title).toMatch(/[A-Za-z]/);
  });
  it("localizes default presets while preserving typed variation text",()=>{
    render(<LocaleProvider locale="ru"><PublicExamplePage example={flowerExample}/></LocaleProvider>);
    const expected=localizePublicData({presets:[{label:"Warmer colors"} ]},"ru").presets[0].label;
    expect(expected).not.toBe("Warmer colors");
    fireEvent.click(screen.getByRole("button",{name:expected}));
    const input=screen.getByRole("textbox");
    fireEvent.change(input,{target:{value:"Keep my exact USER instruction"}});
    expect(input).toHaveValue("Keep my exact USER instruction");
  });
  it("opens localized workflow steps and retains the locale-aware destination",()=>{
    render(<LocaleProvider locale="ru"><WorkflowLanding workflow={productCampaignWorkflow}/></LocaleProvider>);
    fireEvent.click(screen.getByRole("button",{name:localizePublicData(workflowEn,"ru").hero.action}));
    const dialog=screen.getByRole("dialog",{name:localizePublicData(productCampaignWorkflow,"ru").title});
    expect(within(dialog).getAllByRole("tab")[0]).toHaveTextContent(localizePublicData(productCampaignWorkflow,"ru").steps[0].title);
    expect(within(dialog).getByRole("link",{name:localizePublicData(workflowEn,"ru").continue})).toHaveAttribute("href","/ru/app");
  });
  it("displays translated taxonomy labels while filtering by their original IDs",()=>{
    const {container}=render(<LocaleProvider locale="ru"><PublicExamplePage example={flowerExample}/></LocaleProvider>);
    const trigger=screen.getByRole("button",{name:`${localizePublicData(publicExampleEn,"ru").filterPrefix} Узор`});
    expect(trigger).toHaveTextContent("Узор");
    fireEvent.click(trigger);
    expect(trigger).toHaveAttribute("aria-pressed","true");
    const expected=flowerExample.similar.filter(item=>item.tags.includes("pattern"));
    expect(expected.length).toBeGreaterThan(0);
    expect(container.querySelectorAll('[data-example-related="similar"]')).toHaveLength(expected.length);
    expect(flowerExample.taxonIds).toContain("pattern");
  });
  it("shares the Russian public route while retaining the original slug",()=>{
    const writeText=vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator,"clipboard",{value:{writeText},configurable:true});
    render(<LocaleProvider locale="ru"><PublicExamplePage example={flowerExample}/></LocaleProvider>);
    fireEvent.click(screen.getByRole("button",{name:localizePublicData(publicExampleEn,"ru").share}));
    expect(writeText).toHaveBeenCalledExactlyOnceWith(`${window.location.origin}/ru/inspiration/examples/${flowerExample.slug}`);
  });
});
