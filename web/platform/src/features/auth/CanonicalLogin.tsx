"use client";

import { type FormEvent, useEffect, useRef, useState } from "react";
import { BrandText } from "@/components/brand/BrandName/BrandName";
import { BrandLogo } from "@/components/brand/BrandLogo/BrandLogo";
import { Button } from "@/components/ui/Button/Button";
import { LoadingIndicator, StateNotice } from "@/components/ui/AsyncState/AsyncState";
import { useDictionary, useLocale } from "@/i18n/LocaleProvider";
import { localizeHref, stripLocale } from "@/i18n/routing";
import { classifySignInFailure, completeSignIn, reportSignInFailure, signInFailureText, signInResponseFailure, type SignInStage } from "@/lib/auth/sign-in-completion";
import { defaultAuthMethods, providerNames, safeAuthorizationURL, visibleOAuthProvidersForUI, type AuthMethods, type OAuthProvider } from "@/lib/auth/methods";
import { webBrowserFetch } from "@/lib/web-api/browser";
import type { LegalMetadata } from "@/features/legal/model";
import { CredentialField } from "./CredentialField";
import { PasswordField } from "./PasswordField";
import { VerificationCodeField } from "./VerificationCodeField";
import { EmailRegistration } from "./EmailRegistration";
import { RecoveryFlow } from "./RecoveryFlow";
import styles from "./LoginForm/LoginForm.module.css";

export type AuthScreen="choice"|"password"|"register"|"recover"|"reset"|"verify";
type Props=Readonly<{returnTo?:string;methods?:AuthMethods;preview?:boolean;oauthFailed?:boolean;entry?:"email"|"choice";initialMode?:"login"|"register";legalMetadata?:LegalMetadata;appearance?:"default"|"minimal";initialScreen?:AuthScreen;navigationEnabled?:boolean}>;
const providerOrder:Record<OAuthProvider,number>={yandex:0,vk:1,apple:2,telegram:3,google:4};
function screenURL(screen:AuthScreen){return screen==="choice"?"/login":screen==="password"?"/login?step=password":`/login/${screen}`;}
function screenFromURL():AuthScreen {const path=stripLocale(window.location.pathname);const suffix=path.split("/")[2];if(["register","recover","reset","verify"].includes(suffix))return suffix as AuthScreen;const query=new URLSearchParams(window.location.search);return query.get("mode")==="register"?"register":query.get("step")==="password"?"password":"choice";}
function ProviderIcon({provider}:{provider:OAuthProvider}) {
 if(provider==="yandex")return <svg className={styles.providerMark} width="24" height="24" viewBox="0 0 24 24" aria-hidden="true"><path d="M2.53992 12C2.53992 6.75315 6.79212 2.5 12.0399 2.5C17.2858 2.5 21.5399 6.75315 21.5399 12C21.5399 17.2469 17.2858 21.5 12.0399 21.5C6.79212 21.5 2.53992 17.2469 2.53992 12Z" fill="#FC3F1D"/><path d="M13.256 7.88312H12.3782C10.7689 7.88312 9.92248 8.69822 9.92248 9.89997C9.92248 11.2585 10.5077 11.895 11.7094 12.711L12.7022 13.3798L9.84933 17.6425H7.71753L10.2778 13.8292C8.80528 12.7737 7.97878 11.7487 7.97878 10.0149C7.97878 7.84132 9.49403 6.35742 12.3678 6.35742H15.2206V17.632H13.256V7.88312Z" fill="white"/></svg>;
 if(provider==="vk")return <svg className={styles.providerMark} width="24" height="24" viewBox="0 0 24 24" aria-hidden="true"><path d="M3 11.62C3 7.32078 3 5.17119 4.33559 3.83559C5.67119 2.5 7.82078 2.5 12.12 2.5H12.88C17.1792 2.5 19.3288 2.5 20.6644 3.83559C22 5.17119 22 7.32078 22 11.62V12.38C22 16.6792 22 18.8288 20.6644 20.1644C19.3288 21.5 17.1792 21.5 12.88 21.5H12.12C7.82078 21.5 5.67119 21.5 4.33559 20.1644C3 18.8288 3 16.6792 3 12.38V11.62Z" fill="#0077FF"/><path d="M13.1096 16.1861C8.77918 16.1861 6.30922 13.2173 6.2063 8.27734H8.37547C8.44672 11.9032 10.0458 13.439 11.3125 13.7557V8.27734H13.3551V11.4044C14.6059 11.2698 15.9199 9.84484 16.3633 8.27734H18.4058C18.0654 10.209 16.6404 11.634 15.6271 12.2198C16.6404 12.6948 18.2634 13.9378 18.8809 16.1861H16.6325C16.1496 14.6819 14.9463 13.5182 13.3551 13.3598V16.1861H13.1096Z" fill="white"/></svg>;
 return <span className={styles.providerMark} aria-hidden="true">{providerNames[provider].slice(0,1)}</span>;
}

export function LoginForm({returnTo,methods=defaultAuthMethods,preview=false,oauthFailed=false,entry="email",initialMode="login",legalMetadata,initialScreen,navigationEnabled=false}:Props){
 const t=useDictionary(),locale=useLocale();
 const [screen,setScreen]=useState<AuthScreen>(initialScreen??(initialMode==="register"?"register":entry==="choice"?"choice":"password"));
 const [email,setEmail]=useState("");const [password,setPassword]=useState("");const [code,setCode]=useState("");
 const [persistent,setPersistent]=useState(false);const [hint,setHint]=useState("");const [useRemembered,setUseRemembered]=useState(true);
 const [hintProvider,setHintProvider]=useState<OAuthProvider|null>(null);
 const [pending,setPending]=useState(false),[error,setError]=useState(oauthFailed?t.auth.providerFailure:""),[diagnosticId,setDiagnosticId]=useState("");
 const [accepted,setAccepted]=useState<Response|null>(null),[signInStage,setSignInStage]=useState<SignInStage>("credentials"),[previewDone,setPreviewDone]=useState(false);
 const [resendSeconds,setResendSeconds]=useState(0),[emailTouched,setEmailTouched]=useState(false);
 const request=useRef<AbortController|null>(null),root=useRef<HTMLElement>(null);
 const interactionRevision=useRef(0);
 const remembered=Boolean(hint&&useRemembered&&screen==="choice");
 const validEmail=/^[^\s@]+@[^\s@]+\.[^\s@]+$/u.test(email.trim())&&email.trim().length<=254;
 const providers=visibleOAuthProvidersForUI(methods.providers).sort((a,b)=>providerOrder[a]-providerOrder[b]);
 const formClass=`${styles.form} ${styles.minimalForm}`;
 useEffect(()=>()=>request.current?.abort(),[]);
 useEffect(()=>{root.current?.querySelector<HTMLHeadingElement>("h1")?.focus({preventScroll:true});},[screen,previewDone]);
 useEffect(()=>{if(!resendSeconds)return;const timer=window.setTimeout(()=>setResendSeconds(v=>Math.max(0,v-1)),1000);return()=>window.clearTimeout(timer);},[resendSeconds]);
 useEffect(()=>{if(preview||entry!=="choice"||interactionRevision.current!==0)return;const c=new AbortController();const revision=interactionRevision.current;void webBrowserFetch("/web/v1/auth/remembered",{signal:c.signal}).then(async r=>{if(!r.ok)return;const data=await r.json();if(c.signal.aborted||revision!==interactionRevision.current)return;const provider=methods.providers.find(p=>p===data.provider);if(provider&&visibleOAuthProvidersForUI([provider]).length){setHintProvider(provider);setHint(typeof data.email_hint==="string"&&data.email_hint?data.email_hint:providerNames[provider]);}else if(!data.provider&&typeof data.email_hint==="string"&&data.email_hint.length<255)setHint(data.email_hint);}).catch(()=>{});return()=>c.abort();},[entry,preview,methods.providers]);
 useEffect(()=>{if(!navigationEnabled)return;const pop=()=>{interactionRevision.current+=1;request.current?.abort();request.current=null;setScreen(screenFromURL());setPassword("");setCode("");setPending(false);setError("");setAccepted(null);setPreviewDone(false);};window.addEventListener("popstate",pop);return()=>window.removeEventListener("popstate",pop);},[navigationEnabled]);
 function navigate(next:AuthScreen){interactionRevision.current+=1;request.current?.abort();request.current=null;setScreen(next);setPassword("");setCode("");setError("");setAccepted(null);setPending(false);setPreviewDone(false);setDiagnosticId("");if(navigationEnabled)window.history.pushState(null,"",localizeHref(screenURL(next),locale));}
 async function finish(response:Response,c:AbortController,started:number){setAccepted(response);setPassword("");setCode("");let stage:SignInStage="session";try{await completeSignIn(response,{locale,returnTo,signal:c.signal,onStage:next=>{stage=next;setSignInStage(next);}});}catch(e){if(c.signal.aborted)return;const d=classifySignInFailure(e,stage);reportSignInFailure(d,started);setError(signInFailureText(d,t.auth,t.login.failure));setDiagnosticId(d.diagnosticId);}}
 async function submit(event?:FormEvent<HTMLFormElement>,completedCode=code){
  event?.preventDefault();if(pending||request.current)return;
  if(!accepted&&screen==="verify"&&!/^\d{6}$/.test(completedCode)){setError(t.auth.registrationCodeFailure);return;}
  setPending(true);setError("");setDiagnosticId("");const c=new AbortController();request.current=c;const started=performance.now();
  try{
   if(accepted){await finish(accepted,c,started);return;}
   if(preview){setPassword("");if(screen==="verify")setPreviewDone(true);else{setScreen("verify");setCode("");setResendSeconds(60);if(navigationEnabled)window.history.pushState(null,"",localizeHref("/login/verify",locale));}return;}
   const path=screen==="verify"?"/web/v1/auth/password/verify-login":"/web/v1/auth/password/login";
   const body=screen==="verify"?{code:completedCode}:remembered?{password,persistent,remembered:true}:{email:email.trim(),password,persistent};
   const response=await webBrowserFetch(path,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(body),signal:AbortSignal.any([c.signal,AbortSignal.timeout(15_000)])});
   if(c.signal.aborted)return;
   if(response.status===202){setPassword("");setCode("");setScreen("verify");setResendSeconds(60);if(navigationEnabled)window.history.pushState(null,"",localizeHref("/login/verify",locale));return;}
   if(!response.ok){if(screen==="verify"){setError(response.status===429?t.auth.registrationRateLimited:response.status===400?t.auth.registrationCodeFailure:t.auth.loginUnavailable);return;}throw signInResponseFailure(response,"credentials");}
   await finish(response,c,started);
  }catch(e){if(c.signal.aborted)return;const d=classifySignInFailure(e,"credentials");reportSignInFailure(d,started);setError(signInFailureText(d,t.auth,t.login.failure));setDiagnosticId(d.diagnosticId);}
  finally{if(!c.signal.aborted){setPassword("");setPending(false);request.current=null;}}
 }
 async function resend(){
  if(pending||request.current||resendSeconds)return;
  const c=new AbortController();request.current=c;setPending(true);setError("");
  try{if(!preview){const r=await webBrowserFetch("/web/v1/auth/password/resend-login",{method:"POST",signal:c.signal});if(c.signal.aborted)return;if(!r.ok){setError(r.status===429?t.auth.registrationRateLimited:t.auth.registrationCodeFailure);return;}}if(c.signal.aborted)return;setCode("");setResendSeconds(60);}
  catch{if(!c.signal.aborted)setError(t.auth.loginNetwork);}finally{if(!c.signal.aborted){setPending(false);request.current=null;}}
 }
 async function forget(){
  if(pending||request.current)return;
  const c=new AbortController();request.current=c;setPending(true);setError("");
  try{if(!preview){const r=await webBrowserFetch("/web/v1/auth/forget",{method:"POST",signal:c.signal});if(c.signal.aborted)return;if(!r.ok)throw new Error("forget");}if(c.signal.aborted)return;setUseRemembered(false);setHint("");setHintProvider(null);setEmail("");navigate("choice");}
  catch{if(!c.signal.aborted)setError(t.auth.loginNetwork);}finally{if(!c.signal.aborted){setPending(false);request.current=null;}}
 }
 async function oauth(provider:OAuthProvider){
  if(pending||request.current||preview)return;
  interactionRevision.current+=1;
  const c=new AbortController();request.current=c;setPending(true);setError("");let redirected=false;
  try{const r=await webBrowserFetch(`/web/v1/auth/oauth/${provider}/start`,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({locale}),signal:c.signal});if(c.signal.aborted)return;if(!r.ok)throw new Error("oauth");const data=await r.json();if(c.signal.aborted)return;const target=safeAuthorizationURL(data?.authorization_url,provider);if(!target)throw new Error("oauth");window.location.assign(target);redirected=true;}
  catch{if(!c.signal.aborted)setError(t.auth.providerFailure);}finally{if(!c.signal.aborted&&!redirected){setPending(false);request.current=null;}}
 }
 const errorView=error?<StateNotice inline kind="error">{error}{diagnosticId?<small className={styles.diagnostic}>{t.auth.loginDiagnostic} {diagnosticId}</small>:null}</StateNotice>:null;
 const previewView=preview?<p className={styles.previewNote}>{t.auth.preview}</p>:null;
 const emailField=<CredentialField label={t.login.emailLabel} id="login-email" name="email" autoComplete="email" type="email" placeholder={t.auth.minimalEmailPlaceholder} maxLength={254} required disabled={pending} value={email} onBlur={()=>setEmailTouched(true)} onChange={e=>{interactionRevision.current+=1;setEmail(e.target.value);}} hint={emailTouched&&email&&!validEmail?t.auth.invalidEmail:undefined}/>;
 const staySigned=<label className={styles.keepSigned}><input type="checkbox" checked={persistent} disabled={pending} onChange={e=>setPersistent(e.target.checked)}/>{t.auth.staySigned}</label>;
 if(screen==="recover"||screen==="reset")return <section ref={root} className={formClass} aria-labelledby="recovery-title"><RecoveryFlow key={screen} mode={screen==="reset"?"reset":"request"} preview={preview} initialEmail={email} onEmailChange={setEmail} onBack={()=>navigate("password")}/></section>;
 if(screen==="register")return <section ref={root} className={formClass} aria-labelledby="login-title"><EmailRegistration preview={preview} returnTo={returnTo} legalMetadata={legalMetadata} initialEmail={email} passwordFirst title={t.auth.registrationEntry} onEmailChange={setEmail} onBack={()=>navigate("choice")} onSignIn={value=>{setEmail(value);navigate("password");}}/></section>;
 if(screen==="choice"&&!remembered)return <section ref={root} className={`${styles.choiceEntry} ${styles.minimal}`} aria-labelledby="login-title"><div className={styles.choiceCard}>
  <h1 id="login-title" tabIndex={-1}><BrandText>{t.auth.minimalWelcome}</BrandText></h1>
  {providers.length?<><p className={styles.providerLabel}>{t.auth.minimalProvidersLabel}</p><div className={styles.choiceProviders}>{providers.map(provider=><Button key={provider} className={styles.providerPill} disabled={pending||preview} onClick={()=>void oauth(provider)}><ProviderIcon provider={provider}/>{provider==="yandex"?t.auth.yandexEntry:provider==="vk"?t.auth.vkEntry:`${t.auth.providerEntry} ${providerNames[provider]}`}</Button>)}</div><div className={styles.divider}><span>{t.auth.minimalEmailDivider}</span></div></>:null}
  <form className={styles.emailChoice} onSubmit={e=>{e.preventDefault();if(validEmail&&!pending){setEmail(v=>v.trim());navigate("password");}}}>{emailField}<Button className={styles.continueButton} type="submit" disabled={pending||!methods.password||!validEmail}>{t.auth.minimalContinue}</Button></form>
  {errorView}{methods.registration?<p className={styles.accountPrompt}>{t.auth.minimalSignupPrompt}{" "}<button type="button" className={styles.textAction} disabled={pending} onClick={()=>navigate("register")}>{t.auth.minimalSignupAction}</button></p>:null}{previewView}
 </div></section>;
 if(remembered&&hintProvider)return <section ref={root} className={formClass} aria-labelledby="login-title"><header className={styles.heading}><h1 id="login-title" tabIndex={-1}>{t.auth.welcomeBack}</h1></header><div className={styles.remembered}><BrandLogo size={40}/><p>{hint}</p></div><Button disabled={pending||preview} onClick={()=>void oauth(hintProvider)}><ProviderIcon provider={hintProvider}/>{hintProvider==="yandex"?t.auth.yandexEntry:hintProvider==="vk"?t.auth.vkEntry:`${t.auth.providerEntry} ${providerNames[hintProvider]}`}</Button>{errorView}<button type="button" disabled={pending} className={styles.textAction} onClick={()=>void forget()}>{t.auth.useAnotherAccount}</button>{previewView}</section>;
 return <section ref={root} className={formClass} aria-labelledby="login-title">
  <button type="button" className={`${styles.textAction} ${styles.backAction}`} disabled={pending} onClick={()=>{setUseRemembered(false);navigate("choice");}}>{t.auth.otherMethods}</button>
  <header className={styles.heading}><h1 id="login-title" tabIndex={-1}><BrandText>{screen==="verify"?t.auth.verifySigninTitle:remembered?t.auth.welcomeBack:t.auth.minimalWelcome}</BrandText></h1></header>
  {previewView}
  {previewDone?<><p role="status">{t.auth.loginPreviewComplete}</p><Button onClick={()=>navigate("choice")}>{t.auth.back}</Button></>:<form className={styles.fields} onSubmit={e=>void submit(e)}>
   {accepted?null:screen==="verify"?<><p className={styles.hint} role="status">{preview?t.auth.registrationPreviewCode:t.auth.registrationCodeSent}</p><VerificationCodeField value={code} onChange={setCode} onComplete={value=>void submit(undefined,value)} disabled={pending} invalid={Boolean(error)} /></>:<>
    {remembered?<div className={styles.remembered}><BrandLogo size={40}/><p>{hint}</p></div>:emailField}
    <PasswordField id="login-password" value={password} onChange={setPassword} disabled={pending}/>{staySigned}
   </>}
   {errorView}
   <Button type="submit" disabled={pending||(!accepted&&screen==="verify"&&!/^\d{6}$/.test(code))||(!accepted&&screen!=="verify"&&(!password||(!remembered&&!validEmail)||!methods.password))}>{pending?<><LoadingIndicator label=""/>{accepted?signInStage==="navigation"?t.auth.loginOpening:t.auth.loginConfirming:t.auth.pending}</>:accepted?t.auth.loginRetry:screen==="verify"?t.auth.verifyRegistrationEmail:t.login.submitLabel}</Button>
   {screen==="verify"&&!accepted?<><button type="button" className={styles.textAction} disabled={pending||resendSeconds>0} onClick={()=>void resend()}>{t.auth.resend}{resendSeconds?` (${resendSeconds})`:""}</button><p className={styles.accountPrompt}>{t.auth.mailHelpShort}{" "}<a href="mailto:support@epifera.com">{t.legal.support}</a></p><button type="button" className={styles.textAction} disabled={pending} onClick={()=>navigate("password")}>{t.auth.changeEmail}</button></>:null}
   {screen!=="verify"&&!accepted&&methods.recovery?<button type="button" className={styles.textAction} disabled={pending} onClick={()=>navigate("recover")}>{t.auth.forgot}</button>:null}
   {remembered?<button type="button" className={styles.textAction} disabled={pending} onClick={()=>void forget()}>{t.auth.useAnotherAccount}</button>:null}
  </form>}
  {screen!=="verify"&&methods.registration&&!accepted?<p className={styles.accountPrompt}>{t.auth.minimalSignupPrompt}{" "}<button type="button" className={styles.textAction} disabled={pending} onClick={()=>navigate("register")}>{t.auth.minimalSignupAction}</button></p>:null}
 </section>;
}
