/** Existing account UI examples, not an authenticated payment catalog or sale offer. */
export const subscriptionPreviewPlans=[
 {id:"lite",name:"Lite",amount:19900,stars:400,period:"week",allocation:"one-time"},
 {id:"start",name:"Start+",amount:54900,stars:1250,period:"month",allocation:"monthly"},
 {id:"pro",name:"Pro",amount:99900,stars:2250,period:"month",allocation:"monthly"},
 {id:"ultima",name:"Ultima",amount:199900,stars:4800,period:"month",allocation:"monthly"},
 {id:"elite",name:"Elite",amount:499900,stars:12000,period:"month",allocation:"monthly"},
] as const;
export type SubscriptionPreviewPlan=typeof subscriptionPreviewPlans[number];
export type SubscriptionPreviewId=SubscriptionPreviewPlan["id"];
export function subscriptionPlanPreview(id:SubscriptionPreviewId){return subscriptionPreviewPlans.find(plan=>plan.id===id)!;}
/** Preserve the existing account dialog's string presentation without changing its purchase flow. */
export function subscriptionPlanDialogValues(id:SubscriptionPreviewId){const plan=subscriptionPlanPreview(id);return{name:plan.name,price:String(plan.amount/100),credits:String(plan.stars)};}
