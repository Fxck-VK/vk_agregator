"use client";
import {useLocale} from '@/i18n/LocaleProvider';
import {localizePublicData} from './public-copy';
import {bindTemplateSources,useTemplateData} from '@/content/templates/TemplateDataProvider';

export function usePublicCopy<T>(source:T,bindings?:Readonly<Record<string,string>>):T{
  const data=useTemplateData();
  return localizePublicData(bindTemplateSources(source,bindings,data),useLocale());
}
