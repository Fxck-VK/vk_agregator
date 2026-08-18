import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { publicDictionaryRu } from "@/i18n/public/ru";

import { PublicShell } from "./PublicShell";

describe("PublicShell", () => {
  it("renders an independent public header, main region, and footer", () => {
    render(<PublicShell dictionary={publicDictionaryRu}><h1>Публичная страница</h1></PublicShell>);

    expect(screen.getByRole("banner")).toBeInTheDocument();
    expect(screen.getByRole("main")).toContainElement(screen.getByRole("heading", { level: 1 }));
    expect(screen.getByRole("contentinfo")).toBeInTheDocument();
    expect(screen.queryByTestId("app-shell")).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: publicDictionaryRu.navigation.openWorkspace })).toHaveAttribute(
      "href",
      "/app",
    );
  });
});
