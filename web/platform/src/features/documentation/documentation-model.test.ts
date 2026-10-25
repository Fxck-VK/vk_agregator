import {describe,expect,it} from "vitest";
import {documentationArticles} from "@/i18n/public/documentation-en";
import {documentationNeighbours,findDocumentationArticle,searchDocumentation} from "./documentation-model";

describe("documentation catalogue",()=>{
  it("resolves only exact article slugs",()=>{
    expect(findDocumentationArticle("account-management")?.title).toBe("Account management");
    expect(findDocumentationArticle("unknown")).toBeUndefined();
    expect(findDocumentationArticle("../account-management")).toBeUndefined();
    expect(new Set(documentationArticles.map(a=>a.slug)).size).toBe(documentationArticles.length);
  });
  it("searches titles, sections and body with trimmed case-insensitive words",()=>{
    expect(searchDocumentation("  BACKUP EMAIL  ").map(a=>a.slug)).toContain("account-management");
    expect(searchDocumentation("spam").map(a=>a.slug)).toContain("receiving-emails");
    expect(searchDocumentation("qzx-not-a-topic")).toEqual([]);
    expect(searchDocumentation("   ")).toHaveLength(documentationArticles.length);
  });
  it("looks up and searches the requested editorial language without changing slugs",()=>{
    expect(findDocumentationArticle("account-management","ru")?.title).toBe("Управление аккаунтом");
    expect(searchDocumentation("  РЕЗЕРВНЫЙ АДРЕС  ","ru").map(a=>a.slug)).toContain("account-management");
    expect(searchDocumentation("спам","ru").map(a=>a.slug)).toContain("receiving-emails");
    expect(searchDocumentation("резервный","en")).toEqual([]);
    expect(searchDocumentation("   ","ru")).toHaveLength(documentationArticles.length);
    expect(findDocumentationArticle("../account-management","ru")).toBeUndefined();
  });
  it("does not wrap article neighbours at catalogue boundaries",()=>{
    expect(documentationNeighbours(documentationArticles[0].slug).previous).toBeUndefined();
    expect(documentationNeighbours(documentationArticles.at(-1)!.slug).next).toBeUndefined();
    const {previous,next}=documentationNeighbours("account-management");
    expect(previous?.slug).toBe("troubleshooting");
    expect(next?.slug).toBe("receiving-emails");
    expect(documentationNeighbours("unknown")).toEqual({});
  });
  it("localizes neighbouring titles while keeping catalogue boundaries",()=>{
    const {previous,next}=documentationNeighbours("account-management","ru");
    expect(previous?.title).toBe("Решение проблем");
    expect(next?.title).toBe("Доставка писем");
    expect(previous?.slug).toBe("troubleshooting");
    expect(documentationNeighbours(documentationArticles[0].slug,"ru").previous).toBeUndefined();
    expect(documentationNeighbours("unknown","ru")).toEqual({});
  });
  it("keeps in-article anchors unique and related help links resolvable",()=>{
    for(const article of documentationArticles){
      expect(new Set(article.sections.map(s=>s.id)).size).toBe(article.sections.length);
      for(const section of article.sections)for(const block of section.blocks){
        if(block.type!=="paragraph")continue;
        for(const part of block.content)if(typeof part!=="string"&&part.href.startsWith("/ai/docs/"))expect(findDocumentationArticle(part.href.slice("/ai/docs/".length))).toBeDefined();
      }
    }
  });
});
