import type { Metadata } from "next";
import type { ReactNode } from "react";

import { PublicShell } from "@/components/public/PublicShell/PublicShell";
import { publicDictionaryRu } from "@/i18n/public/ru";

export const metadata: Metadata = {
  robots: {
    index: true,
    follow: true,
  },
};

export default function PublicLayout({ children }: Readonly<{ children: ReactNode }>) {
  return <PublicShell dictionary={publicDictionaryRu}>{children}</PublicShell>;
}
