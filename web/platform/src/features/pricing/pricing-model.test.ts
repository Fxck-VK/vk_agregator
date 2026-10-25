import {describe,expect,it} from "vitest";
import {subscriptionPreviewPlans} from "./subscription-preview";
import {budgetFor,parseTeamSeats,formatPricingAmount,buildPricingEnquiry} from "./pricing-model";
describe("pricing preview arithmetic and validated enquiry inputs",()=>{
 it("preserves all five current UI base amounts and star allocations",()=>{expect(subscriptionPreviewPlans.map(p=>[p.id,p.amount,p.stars,p.period])).toEqual([['lite',19900,400,'week'],['start',54900,1250,'month'],['pro',99900,2250,'month'],['ultima',199900,4800,'month'],['elite',499900,12000,'month']]);});
 it("projects twelve monthly amounts without inventing discounts or annual allocations",()=>{const p=subscriptionPreviewPlans[1];expect(budgetFor(p,'monthly')).toBe(54900);expect(budgetFor(p,'yearly')).toBe(658800);expect(budgetFor(subscriptionPreviewPlans[0],'yearly')).toBeNull();});
 it("keeps source data immutable and uses integer kopecks",()=>{for(const p of subscriptionPreviewPlans.filter(p=>p.period==='month')){expect(Number.isSafeInteger(budgetFor(p,'yearly'))).toBe(true);expect(budgetFor(p,'yearly')).toBe(p.amount*12);}expect(subscriptionPreviewPlans[2].stars).toBe(2250);expect(formatPricingAmount(54900)).toContain('549');});
 it("formats the same ruble amount for the explicit display locale",()=>{expect(formatPricingAmount(199900,'en')).toBe('1,999');expect(formatPricingAmount(199900,'ru')).toBe('1\u00a0999');expect(formatPricingAmount(199900)).toBe('1,999');});
 it("builds a Russian editorial enquiry while preserving the team count",()=>{const quote=buildPricingEnquiry(20,false,'ru');expect(quote).toContain('Запрос для команды в Epifera');expect(quote).toContain('Размер команды: 20 чел.');expect(quote).toContain('творческим инструментам');expect(quote).not.toMatch(/Team|people|Price|Please|requirements/);expect(buildPricingEnquiry(20,true,'ru')).toContain('Корпоративный запрос');expect(buildPricingEnquiry(20,false,'en')).toContain('Team size: 20 people');});
 it.each(['','1','101','2.5','-2','NaN','Infinity','1e2',' 2 '])("rejects invalid team seats %s",value=>{expect(parseTeamSeats(value)).toBeNull();});
 it.each(['2','20','100'])("accepts bounded integer seats %s",value=>{expect(parseTeamSeats(value)).toBe(Number(value));});
});
