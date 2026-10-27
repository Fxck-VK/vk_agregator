import type { Metadata } from "next";
import { getRequestLocale, redirectLocalized } from "@/i18n/server";
import type { LoginSearchParams } from "@/features/auth/login-page.server";
export const metadata:Metadata={robots:{index:false,follow:false}};
export default async function Page({searchParams}:{searchParams?:LoginSearchParams}={}){
 const query=await searchParams;const locale=await getRequestLocale();
 const path=query?.mode==="register"?"/login/register":"/login";
 const safe=new URLSearchParams();
 if(query?.oauth==="failed")safe.set("oauth","failed");
 if(query?.step==="password"&&path==="/login")safe.set("step","password");
 redirectLocalized(`${path}${safe.size?`?${safe}`:""}`, locale);
}
