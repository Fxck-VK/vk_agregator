import {fireEvent,render,screen,waitFor,within} from "@testing-library/react";
import {afterEach,describe,expect,it,vi} from "vitest";
import {WorkspaceAccountProvider} from "@/features/account/WorkspaceAccount/WorkspaceAccount";
import {createSubscriptionCheckout,loadSubscription} from "@/features/subscriptions/subscriptions";
import {SubscriptionPlansDialog} from "./SubscriptionPlansDialog";
const actions=vi.hoisted(()=>({open:vi.fn(),push:vi.fn()}));
vi.mock("next/navigation",()=>({useRouter:()=>({push:actions.push})}));
vi.mock("@/features/payments/checkout-navigation",()=>({checkoutNavigation:{open:actions.open}}));
vi.mock("@/features/payments/PaymentStatus",()=>({PaymentStatus:()=> <div>Проверка оплаты сервером</div>}));
vi.mock("@/features/subscriptions/subscriptions",async original=>({...await original<typeof import("@/features/subscriptions/subscriptions")>(),loadSubscription:vi.fn(),createSubscriptionCheckout:vi.fn()}));
const state={subscription:null,balances:[],checkout_available:true,recurring_available:true,trial_used:false};
const profile={account_id:"10000000-0000-4000-8000-000000000001",identity_refs:[]};
const payment={id:"20000000-0000-4000-8000-000000000001",amount:54900,credits:1250,currency:"RUB" as const,status:"waiting_for_user" as const,confirmation_url:"https://yookassa.ru/checkout/test"};
afterEach(()=>{vi.resetAllMocks();sessionStorage.clear();});
function show(){return render(<WorkspaceAccountProvider snapshot={{profile,balance:100}}><SubscriptionPlansDialog onClose={vi.fn()}/></WorkspaceAccountProvider>);}
function buy(){const card=screen.getByRole("heading",{name:"Start+"}).closest("article")!;return within(card).getByRole("button",{name:"Активировать подписку"});}
describe("direct subscription purchase",()=>{
 it("starts checkout from the tariff card without a second conditions dialog",async()=>{
  vi.mocked(loadSubscription).mockResolvedValue(state);vi.mocked(createSubscriptionCheckout).mockResolvedValue(payment);show();
  await waitFor(()=>expect(buy()).toBeEnabled());fireEvent.click(buy());
  await waitFor(()=>expect(createSubscriptionCheckout).toHaveBeenCalledExactlyOnceWith({plan_code:"start",auto_renew:false},expect.any(String)));
  expect(actions.open).toHaveBeenCalledExactlyOnceWith(payment.confirmation_url);
  expect(screen.queryByRole("dialog",{name:"Условия подписки"})).not.toBeInTheDocument();
 });
 it("sends renewal and retry policy only after explicit opt-in in the tariff window",async()=>{
  vi.mocked(loadSubscription).mockResolvedValue(state);vi.mocked(createSubscriptionCheckout).mockResolvedValue(payment);show();
  const checkbox=await screen.findByRole("checkbox",{name:"Автопродление подписки"});expect(checkbox).not.toBeChecked();
  fireEvent.click(checkbox);
  const details=screen.getByText("Подробнее об автопродлении").closest("details")!;
  expect(details).not.toHaveAttribute("open");fireEvent.click(within(details).getByText("Подробнее об автопродлении"));
  expect(details).toHaveAttribute("open");expect(details).toHaveTextContent(/Lite → Start\+.*549/);
  expect(details).toHaveTextContent(/14 дней/);expect(details).toHaveTextContent(/не менее 24 часов/);
  fireEvent.click(buy());
  await waitFor(()=>expect(createSubscriptionCheckout).toHaveBeenCalledWith({plan_code:"start",auto_renew:true,retry_policy_version:1},expect.any(String)));
  expect(screen.getByRole("link",{name:"Условия оплаты и автопродления"})).toHaveAttribute("href","/ru/legal/paid-access");
 });
 it("blocks checkout when the server says payment is unavailable",async()=>{
  vi.mocked(loadSubscription).mockResolvedValue({...state,checkout_available:false});show();
  await waitFor(()=>expect(loadSubscription).toHaveBeenCalled());expect(buy()).toBeDisabled();fireEvent.click(buy());
  expect(await screen.findByText("Оплата подписок сейчас недоступна.")).toBeInTheDocument();
  expect(createSubscriptionCheckout).not.toHaveBeenCalled();expect(screen.queryByRole("dialog",{name:"Условия подписки"})).not.toBeInTheDocument();
 });
 it("keeps stable retries without opening another window",async()=>{
  vi.mocked(loadSubscription).mockResolvedValue(state);vi.mocked(createSubscriptionCheckout).mockRejectedValue(new Error("offline"));show();
  await waitFor(()=>expect(buy()).toBeEnabled());fireEvent.click(buy());await screen.findByRole("alert");fireEvent.click(buy());
  await waitFor(()=>expect(createSubscriptionCheckout).toHaveBeenCalledTimes(2));
  expect(vi.mocked(createSubscriptionCheckout).mock.calls[0]).toEqual(vi.mocked(createSubscriptionCheckout).mock.calls[1]);expect(actions.open).not.toHaveBeenCalled();
 });
 it.each([{amount:99900},{credits:2100}])("rejects a price or star mismatch: %j",async mismatch=>{
  vi.mocked(loadSubscription).mockResolvedValue(state);vi.mocked(createSubscriptionCheckout).mockResolvedValue({...payment,...mismatch});show();
  await waitFor(()=>expect(buy()).toBeEnabled());fireEvent.click(buy());
  expect(await screen.findByRole("alert")).toHaveTextContent(/Условия изменились/);
  expect(actions.open).not.toHaveBeenCalled();
 });
 it("allows only one request while a card purchase is pending",async()=>{
  let finish!:(value:typeof payment)=>void;
  vi.mocked(loadSubscription).mockResolvedValue(state);vi.mocked(createSubscriptionCheckout).mockImplementation(()=>new Promise(resolve=>{finish=resolve;}));show();
  await waitFor(()=>expect(buy()).toBeEnabled());const button=buy();fireEvent.click(button);fireEvent.click(button);
  await waitFor(()=>expect(createSubscriptionCheckout).toHaveBeenCalledTimes(1));expect(button).toBeDisabled();
  finish(payment);await waitFor(()=>expect(actions.open).toHaveBeenCalledTimes(1));
 });
 it("ignores a late checkout response after the account changes",async()=>{
  let finish!:(value:typeof payment)=>void;
  vi.mocked(loadSubscription).mockResolvedValue(state);vi.mocked(createSubscriptionCheckout).mockImplementation(()=>new Promise(resolve=>{finish=resolve;}));const view=show();
  await waitFor(()=>expect(buy()).toBeEnabled());fireEvent.click(buy());
  await waitFor(()=>expect(createSubscriptionCheckout).toHaveBeenCalledTimes(1));
  view.rerender(<WorkspaceAccountProvider snapshot={{profile:{...profile,account_id:"10000000-0000-4000-8000-000000000002"},balance:0}}><SubscriptionPlansDialog onClose={vi.fn()}/></WorkspaceAccountProvider>);
  finish(payment);await waitFor(()=>expect(buy()).toBeEnabled());expect(actions.open).not.toHaveBeenCalled();
 });
 it.each(["canceled","failed","succeeded"] as const)("defers a %s outcome to server payment status",async status=>{
  vi.mocked(loadSubscription).mockResolvedValue(state);vi.mocked(createSubscriptionCheckout).mockResolvedValue({...payment,status});show();
  await waitFor(()=>expect(buy()).toBeEnabled());fireEvent.click(buy());
  expect(await screen.findByText("Проверка оплаты сервером")).toBeInTheDocument();expect(actions.open).not.toHaveBeenCalled();
  expect(screen.queryByRole("dialog",{name:"Условия подписки"})).not.toBeInTheDocument();
 });
});
