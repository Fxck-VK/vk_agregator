import type {SubscriptionPreviewPlan} from "./subscription-preview";
import {brandName} from "@/config/brand";
import type {Locale} from "@/i18n/locales";
import {translatePublicText} from "@/i18n/public/public-copy";
export type PricingPeriod="monthly"|"yearly";
export function budgetFor(plan:SubscriptionPreviewPlan,period:PricingPeriod):number|null{return period==="monthly"?plan.amount:null;}
export function formatPricingAmount(amount:number,locale:Locale="en"){return new Intl.NumberFormat(locale==="ru"?"ru-RU":"en-US",{maximumFractionDigits:0}).format(amount/100);}
export function parseTeamSeats(value:string):number|null{if(!/^\d+$/.test(value))return null;const seats=Number(value);return Number.isSafeInteger(seats)&&seats>=2&&seats<=100?seats:null;}
/** Only fixed editorial copy is translated; the numeric seat count stays data. */
export function buildPricingEnquiry(seats:number,enterprise:boolean,locale:Locale="en"){
  return [
    translatePublicText(enterprise?"Enterprise enquiry for {brand}":"Team enquiry for {brand}",locale).replace("{brand}",brandName),
    translatePublicText("Team size: {seats} people",locale).replace("{seats}",new Intl.NumberFormat(locale).format(seats)),
    translatePublicText("Price to be agreed",locale),
    translatePublicText("Please discuss creative tools, access, capacity and support requirements.",locale),
  ].join("\n");
}
