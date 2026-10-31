import {cleanup,fireEvent,render,screen,waitFor,within} from "@testing-library/react";
import {afterEach,describe,expect,it,vi} from "vitest";
import {WorkspaceAccountProvider} from "@/features/account/WorkspaceAccount/WorkspaceAccount";
import {SubscriptionPlansDialog} from "@/components/layout/WorkspaceHeader/SubscriptionPlansDialog";
import {WorkspaceLanding} from "@/features/workspace/WorkspaceLanding/WorkspaceLanding";
import {loadSubscription,subscriptionStateSchema} from "./subscriptions";
vi.mock("./subscriptions",async original=>({...await original<typeof import("./subscriptions")>(),loadSubscription:vi.fn()}));
vi.mock("@/features/workspace/WorkspaceHero/WorkspaceHero",()=>({WorkspaceHero:()=>null}));
vi.mock("next/navigation",()=>({useRouter:()=>({push:vi.fn()})}));
const profile={account_id:"10000000-0000-4000-8000-000000000001",identity_refs:[]};
const state={subscription:null,balances:[],checkout_available:true,recurring_available:false,trial_used:false};
function show(home=false){return render(<WorkspaceAccountProvider snapshot={{profile,balance:0}}>{home?<WorkspaceLanding/>:<SubscriptionPlansDialog onClose={()=>{}}/>}</WorkspaceAccountProvider>);}
afterEach(()=>{cleanup();vi.resetAllMocks();});
describe("account-owned Lite offer",()=>{
  it("requires an explicit trial eligibility flag and does not guess from an empty active subscription",()=>{
    expect(subscriptionStateSchema.parse(state).trial_used).toBe(false);
    const unknown={subscription:null,balances:[],checkout_available:true,recurring_available:false};
    expect(subscriptionStateSchema.safeParse(unknown).success).toBe(false);
  });
  it("does not flash Lite while the account's eligibility is unknown",async()=>{
    let finish!:(value:typeof state)=>void;
    vi.mocked(loadSubscription).mockReturnValue(new Promise(resolve=>{finish=resolve;}));
    show();
    expect(screen.queryByRole("heading",{name:"Lite"})).toBeNull();
    expect(screen.getByRole("heading",{name:"Start+"})).toBeVisible();
    await waitFor(()=>expect(loadSubscription).toHaveBeenCalled());finish(state);
    expect(await screen.findByRole("heading",{name:"Lite"})).toBeVisible();
  });
  it("hides only Lite after consumption even without an active subscription, including after remount",async()=>{
    vi.mocked(loadSubscription).mockResolvedValue({...state,trial_used:true});
    let view=show();await waitFor(()=>expect(loadSubscription).toHaveBeenCalled());
    await waitFor(()=>expect(screen.getByRole("dialog").querySelectorAll("article")).toHaveLength(5));
    expect(screen.queryByRole("heading",{name:"Lite"})).toBeNull();
    for(const name of ["Start+","Pro","Ultima","Elite","Командный тариф"])expect(screen.getByRole("heading",{name})).toBeVisible();
    view.unmount();view=show();
    expect(screen.queryByRole("heading",{name:"Lite"})).toBeNull();
    await waitFor(()=>expect(loadSubscription).toHaveBeenCalledTimes(2));view.unmount();
  });
  it("parses the server currency format for a paid subscription without losing trial usage",()=>{
    const paid={...state,trial_used:true,subscription:{plan_code:"lite",plan_name:"Lite",amount:19900,stars:400,currency:"rub",period_start:"2026-10-01T00:00:00Z",paid_until:"2026-10-08T00:00:00Z",next_plan_code:"start",next_amount:54900,auto_renew:false,status:"expired"}};
    expect(subscriptionStateSchema.parse(paid)).toMatchObject({trial_used:true,subscription:{currency:"RUB",status:"expired"}});
  });
  it("keeps Lite hidden after a failed account read",async()=>{
    vi.mocked(loadSubscription).mockRejectedValue(new Error("Unavailable"));
    show();await waitFor(()=>expect(loadSubscription).toHaveBeenCalled());
    expect(screen.queryByRole("heading",{name:"Lite"})).toBeNull();
  });
  it("rechecks on checkout close and returns focus to the tariff close button if Lite disappears",async()=>{
    vi.mocked(loadSubscription).mockResolvedValue(state);
    show();const card=(await screen.findByRole("heading",{name:"Lite"})).closest("article")!;
    fireEvent.click(within(card).getByRole("button",{name:"Активировать подписку"}));
    const checkout=screen.getByRole("dialog",{name:"Условия подписки"});
    vi.mocked(loadSubscription).mockResolvedValue({...state,trial_used:true});
    fireEvent.click(within(checkout).getByRole("button",{name:"Закрыть тарифы"}));
    fireEvent.animationEnd(screen.getByTestId("subscription-checkout-backdrop"),{animationName:"modalBackdropOut"});
    await waitFor(()=>expect(screen.queryByRole("heading",{name:"Lite"})).toBeNull());
    expect(screen.getByRole("button",{name:"Закрыть тарифы"})).toHaveFocus();
  });
  it("also removes the consumed Lite promotion from the account home",async()=>{
    vi.mocked(loadSubscription).mockResolvedValue({...state,trial_used:true});
    show(true);await waitFor(()=>expect(loadSubscription).toHaveBeenCalled());
    expect(screen.queryByRole("heading",{name:"Lite"})).toBeNull();
  });
  it("rechecks the home offer after a refreshed server account snapshot",async()=>{
    vi.mocked(loadSubscription).mockResolvedValue(state);
    const view=show(true);await screen.findByRole("heading",{name:"Lite"});
    vi.mocked(loadSubscription).mockResolvedValue({...state,trial_used:true});
    view.rerender(<WorkspaceAccountProvider snapshot={{profile:{...profile},balance:400}}><WorkspaceLanding/></WorkspaceAccountProvider>);
    await waitFor(()=>expect(screen.queryByRole("heading",{name:"Lite"})).toBeNull());
    await waitFor(()=>expect(loadSubscription).toHaveBeenCalledTimes(2));
  });
  it("keeps the unchanged guest tariff cards visible without reading private account state",()=>{
    render(<SubscriptionPlansDialog onClose={()=>{}}/>);
    expect(screen.getByRole("heading",{name:"Lite"})).toBeVisible();
    expect(screen.getByRole("dialog").querySelectorAll("article")).toHaveLength(6);
    expect(loadSubscription).not.toHaveBeenCalled();
    const card=screen.getByRole("heading",{name:"Lite"}).closest("article")!;
    fireEvent.click(within(card).getByRole("button",{name:"Активировать подписку"}));
    fireEvent.click(within(screen.getByRole("dialog",{name:"Условия подписки"})).getByRole("button",{name:"Закрыть тарифы"}));
    fireEvent.animationEnd(screen.getByTestId("subscription-checkout-backdrop"),{animationName:"modalBackdropOut"});
    expect(loadSubscription).not.toHaveBeenCalled();
  });
});
