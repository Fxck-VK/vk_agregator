import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { LocaleProvider } from "@/i18n/LocaleProvider";
import { webBrowserFetch, webBrowserMutation } from "@/lib/web-api/browser";
import { LegalConsentGate } from "./LegalConsentGate";
import { LegalNotice } from "./LegalLinks";

vi.mock("@/lib/web-api/browser",()=>({webBrowserFetch:vi.fn(),webBrowserMutation:vi.fn()}));
const route=vi.hoisted(()=>({path:"/app"}));
vi.mock("@/i18n/navigation",()=>({usePathname:()=>route.path}));
const meta={published:true,version:"2026-10-23.3",checksum:"a".repeat(64),marketing_version:"2026-10-23.1",marketing_checksum:"b".repeat(64)};
const current={...meta,accepted:false,marketing_opt_in:false,marketing_available:true,recipient_version:"d".repeat(64)};
afterEach(()=>{cleanup();vi.resetAllMocks();route.path="/app";});
it("links to the localized agreement and privacy policy without losing the login form",()=>{
 render(<LocaleProvider locale="en"><LegalNotice/></LocaleProvider>);
 expect(screen.getByRole("link",{name:"User agreement"})).toHaveAttribute("href","/en/legal/user-agreement");
 expect(screen.getByRole("link",{name:"Privacy policy"})).toHaveAttribute("target","_blank");
});
it("requires affirmative acceptance, leaves advertising unchecked and saves to the cookie-owned API",async()=>{
 vi.mocked(webBrowserFetch).mockResolvedValue(Response.json(current));
 vi.mocked(webBrowserMutation).mockResolvedValue(Response.json({...current,accepted:true}));
 render(<LegalConsentGate metadata={meta} accountId="account-a"><p>Workspace contents</p></LegalConsentGate>);
 const checkbox=await screen.findByRole("checkbox");
 expect(checkbox).not.toBeChecked();expect(screen.queryByText("Workspace contents")).not.toBeInTheDocument();
 fireEvent.click(screen.getByRole("button",{name:"Принимаю условия и продолжаю"}));
 await screen.findByText("Workspace contents");
 const [path,options]=vi.mocked(webBrowserMutation).mock.calls[0];
 expect(path).toBe("/web/v1/account/legal/accept");
 const body=JSON.parse(String(options.body));
 expect(body).toMatchObject({accepted:true,version:meta.version,checksum:meta.checksum,marketing_opt_in:false});
 expect(body).not.toHaveProperty("account_id");
});
it("refuses to accept a different backend document revision",async()=>{
 vi.mocked(webBrowserFetch).mockResolvedValue(Response.json({...current,checksum:"c".repeat(64)}));
 render(<LegalConsentGate metadata={meta} accountId="account-a"><p>Workspace contents</p></LegalConsentGate>);
 await screen.findByText(/Документы обновились/);
 expect(screen.queryByRole("button",{name:"Принимаю условия и продолжаю"})).not.toBeInTheDocument();
 expect(webBrowserMutation).not.toHaveBeenCalled();
});
it("does not record draft or preview choices",async()=>{
 render(<LegalConsentGate metadata={{...meta,published:false}} accountId="account-a"><p>Workspace contents</p></LegalConsentGate>);
 await waitFor(()=>expect(screen.getByText("Workspace contents")).toBeInTheDocument());
 expect(webBrowserFetch).not.toHaveBeenCalled();expect(webBrowserMutation).not.toHaveBeenCalled();
});
it("allows withdrawing marketing consent even when publication is paused",async()=>{
 vi.mocked(webBrowserMutation).mockResolvedValue(Response.json({...current,published:false,marketing_opt_in:false}));
 render(<LegalConsentGate mode="preferences" metadata={{...meta,published:false}} accountId="account-a"/>);
 fireEvent.click(screen.getByRole("button",{name:"Отозвать согласие на рассылку"}));
 await screen.findByRole("status");
 const [path,options]=vi.mocked(webBrowserMutation).mock.calls[0];
 expect(path).toBe("/web/v1/account/legal/marketing");
 expect(JSON.parse(String(options.body))).toMatchObject({opt_in:false});
});
it("reuses the same idempotency key after a lost response",async()=>{
 vi.mocked(webBrowserFetch).mockResolvedValue(Response.json(current));
 vi.mocked(webBrowserMutation).mockRejectedValueOnce(new Error("lost response")).mockResolvedValueOnce(Response.json({...current,accepted:true}));
 render(<LegalConsentGate metadata={meta} accountId="account-a"><p>Workspace contents</p></LegalConsentGate>);
 const button=await screen.findByRole("button",{name:"Принимаю условия и продолжаю"});
 fireEvent.click(button);
 await screen.findByText(/Не удалось сохранить/);
 fireEvent.click(button);
 await screen.findByText("Workspace contents");
 const calls=vi.mocked(webBrowserMutation).mock.calls;
 expect(calls[0][1].headers).toEqual(calls[1][1].headers);
 expect(calls[0][1].body).toEqual(calls[1][1].body);
});
it("keeps profile security and withdrawal accessible without accepting",()=>{
 route.path="/app/profile";
 vi.mocked(webBrowserFetch).mockResolvedValue(Response.json(current));
 render(<LegalConsentGate metadata={meta} accountId="account-a"><p>Account security</p></LegalConsentGate>);
 expect(screen.getByText("Account security")).toBeInTheDocument();
});
it("does not reuse another account's accepted state",async()=>{
 vi.mocked(webBrowserFetch).mockResolvedValueOnce(Response.json({...current,accepted:true})).mockResolvedValueOnce(Response.json(current));
 const view=render(<LegalConsentGate metadata={meta} accountId="account-a"><p>Workspace contents</p></LegalConsentGate>);
 await screen.findByText("Workspace contents");
 view.rerender(<LegalConsentGate metadata={meta} accountId="account-b"><p>Workspace contents</p></LegalConsentGate>);
 await screen.findByRole("checkbox");
 expect(screen.queryByText("Workspace contents")).not.toBeInTheDocument();
});
it("preserves access to existing files and deletion while asking for current terms",async()=>{
 route.path="/app/files";
 vi.mocked(webBrowserFetch).mockResolvedValue(Response.json(current));
 render(<LegalConsentGate metadata={meta} accountId="account-a"><button>Delete owned file</button></LegalConsentGate>);
 expect(screen.getByRole("button",{name:"Delete owned file"})).toBeEnabled();
 expect(await screen.findByRole("link",{name:"Открыть условия использования"})).toHaveAttribute("href","/ru/app");
});
