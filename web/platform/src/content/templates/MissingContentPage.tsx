"use client";
import {notFound} from '@/i18n/navigation';
/** Retains the existing dev/RSC-safe notFound signal for unregistered URLs. */
export function MissingContentPage(){notFound();return null;}
