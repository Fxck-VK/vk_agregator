import type { Metadata } from "next";
import type { ReactNode } from "react";

import { LocalizedPublicShell } from "@/i18n/public/LocalizedPublicShell";

export const metadata: Metadata = {
  robots: {
    index: true,
    follow: true,
  },
};

export default async function PublicLayout({ children }: Readonly<{ children: ReactNode }>) {
  return <LocalizedPublicShell>{children}</LocalizedPublicShell>;
}
