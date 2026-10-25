import {describe,expect,it} from "vitest";
import {filterMaterials,initialMaterialFilters,packMaterials,paginateMaterials} from "./category-model";
import {natureCategory,natureMaterials} from "@/i18n/public/material-category-en";
describe("material category data and model",()=>{
  it("preserves the real fifty-reference source order",()=>{expect(natureMaterials).toHaveLength(50);expect(new Set(natureMaterials.map(item=>item.id)).size).toBe(50);expect(filterMaterials(natureMaterials,initialMaterialFilters)).toEqual(natureMaterials);expect(natureCategory.mediaType).toBe("photos");});
  it("combines search, source and orientation without claiming unlabeled images are non-AI",()=>{
    const result=filterMaterials(natureMaterials,{...initialMaterialFilters,query:"nature",source:"ai",orientation:"landscape"});expect(result.length).toBeGreaterThan(0);expect(result.every(item=>item.sourceAI&&item.orientation==="landscape")).toBe(true);
    const unknown=filterMaterials(natureMaterials,{...initialMaterialFilters,source:"unspecified"});expect(unknown.every(item=>!item.sourceAI)).toBe(true);
  });
  it("filters actual color/topic metadata and handles zero results",()=>{
    const sample=natureMaterials[0];expect(filterMaterials(natureMaterials,{...initialMaterialFilters,color:sample.color}).every(item=>item.color===sample.color)).toBe(true);
    expect(filterMaterials(natureMaterials,{...initialMaterialFilters,topic:"forest"}).every(item=>item.tags.includes("forest"))).toBe(true);expect(filterMaterials(natureMaterials,{...initialMaterialFilters,query:"zzzzunmatched"})).toEqual([]);
  });
  it("searches localized display text while matching canonical topic and filter IDs",()=>{
    const localizedText=(item:typeof natureMaterials[number])=>`Природа ${item.tags.includes("forest")?"Лес":"Озеро"}`;
    const result=filterMaterials(natureMaterials,{...initialMaterialFilters,query:"лес",topic:"forest"},localizedText);
    expect(result.length).toBeGreaterThan(0);
    expect(result.every(item=>item.tags.includes("forest"))).toBe(true);
    expect(filterMaterials(natureMaterials,{...initialMaterialFilters,query:"Природа"},localizedText)).toEqual(natureMaterials);
    expect(filterMaterials(natureMaterials,{...initialMaterialFilters,query:"лес",topic:"flowers"},localizedText).every(item=>item.tags.includes("flowers"))).toBe(true);
  });
  it("sorts by actual preview area or alphabetically, never by invented dates",()=>{
    const result=filterMaterials(natureMaterials,{...initialMaterialFilters,sort:"largest"});expect(result.map(item=>item.width*item.height)).toEqual(natureMaterials.map(item=>item.width*item.height).sort((a,b)=>b-a));
    const names=filterMaterials(natureMaterials,{...initialMaterialFilters,sort:"name"}).map(item=>item.alt);expect(names).toEqual([...names].sort((a,b)=>a.localeCompare(b,"en")));
  });
  it("paginates real results without duplicates and clamps invalid pages",()=>{const pages=[1,2,3].map(page=>paginateMaterials(natureMaterials,page));expect(pages.map(p=>p.items.length)).toEqual([24,24,2]);expect(pages.flatMap(p=>p.items.map(item=>item.id))).toEqual(natureMaterials.map(item=>item.id));expect(paginateMaterials([],99).page).toBe(1);expect(paginateMaterials(natureMaterials,99).page).toBe(3);});
  it.each([390,820,1280,1600])("packs masonry without overlaps or horizontal overflow at %ipx",viewport=>{
    const width=viewport<640?viewport-40:viewport-136,layout=packMaterials(natureMaterials.slice(0,24),width,viewport,"masonry");expect(layout.cards).toHaveLength(24);
    for(const [i,card]of layout.cards.entries()){expect(card.x).toBeGreaterThanOrEqual(0);expect(card.x+card.width).toBeLessThanOrEqual(width+.01);expect(card.y+card.height).toBeLessThanOrEqual(layout.height+.01);for(const prev of layout.cards.slice(0,i))if(Math.abs(prev.x-card.x)<1)expect(card.y).toBeGreaterThanOrEqual(prev.y+prev.height+23.99);}
    expect(layout.cards.slice(0,layout.columns).every(card=>card.y===0)).toBe(true);
  });
  it("uses the same source order and square cells in square mode",()=>{const layout=packMaterials(natureMaterials.slice(0,8),1100,1280,"square");expect(layout.columns).toBe(4);expect(layout.cards.every(card=>card.width===card.height)).toBe(true);expect(layout.cards[4].y).toBeGreaterThan(0);});
});
