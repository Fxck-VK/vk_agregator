"use client";

import { useLayoutEffect } from "react";

import { readThemePreference, themeBootstrapScript } from "./theme-preference";

export function ThemeBootstrapScript({ nonce }: Readonly<{ nonce?: string }>) {
  useLayoutEffect(() => {
    // Next's error document can mount this root entirely on the client, so the
    // inline server bootstrap may never have run. Restore before that first paint.
    document.documentElement.dataset.theme = readThemePreference();
  }, []);

  return (
    <script
      dangerouslySetInnerHTML={{ __html: themeBootstrapScript }}
      nonce={nonce}
      suppressHydrationWarning
      // Execute before the first paint on the server. A 404 boundary may insert
      // the root again on the client, where React must treat this as inert data.
      type={typeof window === "undefined" ? "text/javascript" : "text/plain"}
    />
  );
}
