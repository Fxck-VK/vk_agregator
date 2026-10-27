import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
vi.mock("@/lib/web-api/browser",()=>({webBrowserFetch:vi.fn()}));
vi.mock("@/lib/auth/sign-in-navigation",()=>({replaceSignInDocument:vi.fn()}));
import { webBrowserFetch } from "@/lib/web-api/browser";
import { replaceSignInDocument } from "@/lib/auth/sign-in-navigation";
import { LoginForm } from "./CanonicalLogin";
const methods={registration:true,password:true,recovery:true,email_link:true,phone_link:false,providers:[]};
beforeEach(()=>{window.history.replaceState(null,"","/ru/login");vi.mocked(webBrowserFetch).mockResolvedValue(Response.json({email_hint:""}));vi.stubGlobal("fetch",vi.fn().mockResolvedValue(Response.json({account_id:"10000000-0000-4000-8000-000000000001",identity_refs:[]})));});
afterEach(()=>{cleanup();vi.clearAllMocks();vi.unstubAllGlobals();});
it("shows the shared password control and sends the real persistent choice",async()=>{
 render(<LoginForm methods={methods} entry="email" />);
 fireEvent.change(screen.getByLabelText("Электронная почта"),{target:{value:"member@example.test"}});
 fireEvent.change(screen.getByLabelText("Пароль"),{target:{value:"synthetic-password"}});
 fireEvent.click(screen.getByRole("button",{name:"Показать пароль"}));
 expect(screen.getByLabelText("Пароль")).toHaveAttribute("type","text");
 fireEvent.click(screen.getByRole("checkbox",{name:"Оставаться в системе"}));
 vi.mocked(webBrowserFetch).mockResolvedValueOnce(new Response(null,{status:201}));
 fireEvent.click(screen.getByRole("button",{name:"Войти"}));
 await vi.waitFor(()=>expect(replaceSignInDocument).toHaveBeenCalled());
 expect(webBrowserFetch).toHaveBeenCalledWith("/web/v1/auth/password/login",expect.objectContaining({body:JSON.stringify({email:"member@example.test",password:"synthetic-password",persistent:true})}));
});
it("waits for the server step-up and never re-sends the password with the code",async()=>{
 render(<LoginForm methods={methods} entry="email" />);
 fireEvent.change(screen.getByLabelText("Электронная почта"),{target:{value:"member@example.test"}});
 fireEvent.change(screen.getByLabelText("Пароль"),{target:{value:"synthetic-password"}});
 vi.mocked(webBrowserFetch).mockResolvedValueOnce(Response.json({status:"verification_required"},{status:202}));
 fireEvent.click(screen.getByRole("button",{name:"Войти"}));
 const first=await screen.findByRole("textbox",{name:"Код из письма: 1"});
 expect(replaceSignInDocument).not.toHaveBeenCalled();
 expect(screen.queryByLabelText("Пароль")).not.toBeInTheDocument();
 fireEvent.paste(first,{clipboardData:{getData:()=>"123456"}});
 vi.mocked(webBrowserFetch).mockResolvedValueOnce(new Response(null,{status:201}));
 fireEvent.click(screen.getByRole("button",{name:"Подтвердить почту"}));
 await vi.waitFor(()=>expect(replaceSignInDocument).toHaveBeenCalled());
 expect(webBrowserFetch).toHaveBeenLastCalledWith("/web/v1/auth/password/verify-login",expect.objectContaining({body:JSON.stringify({code:"123456"})}));
});
it("opens the complete recovery flow and does not send mail in preview",()=>{
 render(<LoginForm methods={methods} entry="email" preview />);
 fireEvent.click(screen.getByRole("button",{name:"Забыли пароль?"}));
 expect(screen.getByRole("heading",{name:"Восстановление пароля"})).toBeInTheDocument();
 expect(webBrowserFetch).not.toHaveBeenCalled();
});

it("does not replace an email being typed with a delayed remembered account",async()=>{
 let resolveRemembered!:(response:Response)=>void;
 vi.mocked(webBrowserFetch).mockReturnValueOnce(new Promise(resolve=>{resolveRemembered=resolve;}));
 render(<LoginForm methods={methods} entry="choice" />);
 fireEvent.change(screen.getByLabelText("Электронная почта"),{target:{value:"other@example.test"}});
 await act(async()=>{resolveRemembered(Response.json({email_hint:"previous@example.test"}));});
 expect(screen.getByLabelText("Электронная почта")).toHaveValue("other@example.test");
 expect(screen.queryByText("previous@example.test")).not.toBeInTheDocument();
});

it("does not restart remembered lookup after typing when provider props change",async()=>{
 vi.mocked(webBrowserFetch).mockResolvedValueOnce(Response.json({email_hint:""}));
 const {rerender}=render(<LoginForm methods={methods} entry="choice" />);
 await act(async()=>{});
 fireEvent.change(screen.getByLabelText("Электронная почта"),{target:{value:"other@example.test"}});
 vi.mocked(webBrowserFetch).mockResolvedValueOnce(Response.json({email_hint:"previous@example.test"}));
 rerender(<LoginForm methods={{...methods,providers:[]}} entry="choice" />);
 await act(async()=>{});
 expect(webBrowserFetch).toHaveBeenCalledTimes(1);
 expect(screen.getByLabelText("Электронная почта")).toHaveValue("other@example.test");
 expect(screen.queryByText("previous@example.test")).not.toBeInTheDocument();
});

it("does not apply a remembered lookup that finishes after navigation",async()=>{
 let resolveRemembered!:(response:Response)=>void;
 vi.mocked(webBrowserFetch).mockReturnValueOnce(new Promise(resolve=>{resolveRemembered=resolve;}));
 render(<LoginForm methods={methods} entry="choice" navigationEnabled />);
 fireEvent.click(screen.getByRole("button",{name:"Зарегистрироваться"}));
 expect(screen.getByRole("heading",{name:"Создать аккаунт"})).toBeInTheDocument();
 await act(async()=>{resolveRemembered(Response.json({email_hint:"previous@example.test"}));});
 fireEvent.click(screen.getByRole("button",{name:"Вернуться ко входу"}));
 expect(screen.getByLabelText("Электронная почта")).toHaveValue("");
 expect(screen.queryByText("previous@example.test")).not.toBeInTheDocument();
});

it("restores the registration query on back and the password step on forward",()=>{
 window.history.replaceState(null,"","/ru/login?mode=register");
 render(<LoginForm methods={methods} entry="choice" initialMode="register" navigationEnabled preview />);
 fireEvent.click(screen.getByRole("button",{name:"Вернуться ко входу"}));
 fireEvent.change(screen.getByLabelText("Электронная почта"),{target:{value:"member@example.test"}});
 fireEvent.click(screen.getByRole("button",{name:"Продолжить"}));
 expect(window.location.search).toBe("?step=password");
 act(()=>{window.history.replaceState(null,"","/ru/login?mode=register");window.dispatchEvent(new PopStateEvent("popstate"));});
 expect(screen.getByRole("heading",{name:"Создать аккаунт"})).toBeInTheDocument();
 act(()=>{window.history.replaceState(null,"","/ru/login?step=password");window.dispatchEvent(new PopStateEvent("popstate"));});
 expect(screen.getByLabelText("Пароль")).toBeInTheDocument();
 expect(screen.getByLabelText("Электронная почта")).toHaveValue("member@example.test");
});

it("allows retry when the browser rejects OAuth navigation",async()=>{
 const assign=vi.fn().mockImplementation(()=>{throw new DOMException("Navigation denied","SecurityError");});
 const actualWindow=window;
 vi.stubGlobal("window",new Proxy(actualWindow,{get(target,key){return key==="location"?{assign}:Reflect.get(target,key,target);}}));
 vi.mocked(webBrowserFetch).mockImplementation(async path=>Response.json(path==="/web/v1/auth/remembered"?{email_hint:""}:{authorization_url:"https://oauth.yandex.ru/authorize?client_id=synthetic"}));
 render(<LoginForm methods={{...methods,providers:["yandex"]}} entry="choice" />);
 const provider=screen.getByRole("button",{name:"Войти с Яндекс ID"});
 fireEvent.click(provider);
 await vi.waitFor(()=>expect(assign).toHaveBeenCalledTimes(1));
 await vi.waitFor(()=>expect(provider).toBeEnabled());
 expect(screen.getByRole("alert")).toBeInTheDocument();
 fireEvent.click(provider);
 await vi.waitFor(()=>expect(assign).toHaveBeenCalledTimes(2));
 expect(webBrowserFetch).toHaveBeenCalledWith("/web/v1/auth/oauth/yandex/start",expect.objectContaining({method:"POST"}));
});
