import type { Metadata } from "next";
import type { ReactNode } from "react";

import { ru } from "@/i18n/ru";

import "./globals.css";

export const metadata: Metadata = {
  title: ru.document.title,
  description: ru.document.description,
};

export default function RootLayout({ children }: Readonly<{ children: ReactNode }>) {
  return (
    <html lang="ru">
      <body>{children}</body>
    </html>
  );
}
