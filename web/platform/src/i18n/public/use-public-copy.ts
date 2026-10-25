"use client";
import {useLocale} from '@/i18n/LocaleProvider';
import {localizePublicData} from './public-copy';

export function usePublicCopy<T>(source:T):T{
  return localizePublicData(source,useLocale());
}
