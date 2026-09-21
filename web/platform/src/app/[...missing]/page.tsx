"use client";

import { notFound } from "@/i18n/navigation";

export default function MissingPage() {
  // Client components are also rendered on the server for an initial request.
  // Keep the HTTP 404 while avoiding React's broken RSC error timing in dev:
  // https://github.com/facebook/react/issues/37561
  notFound();
}
