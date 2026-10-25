"use client";
import type {ReactNode} from "react";
import {HelpCenterFrame} from "@/components/public/help-center/HelpCenterFrame";
import type {HelpRailItem} from "@/components/public/help-center/types";
import type {Locale} from "@/i18n/locales";
import {legalPageText,type LegalNavigationItem} from "@/i18n/public/legal-page";
import {LegalNavigation} from "./LegalNavigation";
import type {LegalSlug} from "./model";
export function LegalPageFrame({locale,documents,currentSlug,children}:{locale:Locale;documents:readonly LegalNavigationItem[];currentSlug?:LegalSlug;children:ReactNode}){
  const t=legalPageText[locale],title=documents.find(doc=>doc.slug===currentSlug)?.title;
  const rail:readonly HelpRailItem[]=[{href:"/app",label:t.newChat,icon:"plus"},{href:"/",label:t.home,icon:"home"},{href:"/app/models",label:t.models,icon:"grid"},{href:"/app/files",label:t.files,icon:"file"},{href:"/legal",label:t.title,icon:"file",current:true,bottom:true},{href:"mailto:support@epifera.com",label:t.support,icon:"support",bottom:true}];
  return <HelpCenterFrame kind="legal" labels={{...t,sectionTitle:t.title,openSectionMenu:t.openLegalMenu}} rail={rail} breadcrumbs={[{label:t.title,href:"/legal"},...(title?[{label:title}]:[])]} readingKey={currentSlug} renderNavigation={onNavigate=><LegalNavigation locale={locale} documents={documents} currentSlug={currentSlug} onNavigate={onNavigate}/>}>{children}</HelpCenterFrame>;
}
