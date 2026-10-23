import { expect, it, vi } from "vitest";
vi.mock("server-only",()=>({}));
import { createLegalCatalog } from "./catalog.server";
import source from "../../../legal/catalog.json";

it("publishes a dated final edition without an inactive tariff proposal or internal audit notes",()=>{
 const text=JSON.stringify(source.documents);
 expect(/На дату проекта|Проект будущей подписки|Тариф проекта|В проверенном коде|В исходной конфигурации|в данном контуре|текущей реализации|согласованного референса|ПРИЛОЖЕНИЕ НЕ ДЕЙСТВУЕТ/.test(text)).toBe(false);
 expect(source.date).toBe("2026-10-23");
 expect(source.documents.find(doc=>doc.slug==="paid-access")?.title).toBe("Условия оплаты и возврата Epifera");
});

it("never publishes incomplete operator details and marks drafts separately",()=>{
 expect(createLegalCatalog("","",true).metadata.published).toBe(false);
 expect(createLegalCatalog("Test","000000000000",true).metadata.published).toBe(false);
 const draft=createLegalCatalog("Synthetic Test Operator","123456789012",false);
 const live=createLegalCatalog("Synthetic Test Operator","123456789012",true);
 expect(live.metadata.checksum).toBe("48fae60b1488ad443a49f80d2376c094847ef6a36ff472921477fa24ccf35da1");
 expect(draft.metadata.checksum).not.toBe(live.metadata.checksum);
 expect(live.metadata.published).toBe(true);
 expect(live.documents).toHaveLength(5);
 expect(JSON.stringify(live.documents)).not.toMatch(/\{(?:NAME|INN)\}/);
 expect(createLegalCatalog("Another Test Operator","123456789012",true).metadata.checksum).not.toBe(live.metadata.checksum);
});
