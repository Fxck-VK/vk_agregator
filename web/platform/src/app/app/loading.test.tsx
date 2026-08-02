import { renderToStaticMarkup } from "react-dom/server";
import { expect, it } from "vitest";

import { ru } from "@/i18n/ru";

import WorkspaceLoading from "./loading";

it("renders a neutral status fallback while workspace navigation is pending", () => {
  const markup = renderToStaticMarkup(<WorkspaceLoading />);

  expect(markup.match(/role="status"/g)).toHaveLength(1);
  expect(markup).toContain(ru.workspace.navigationLoading);
  expect(markup).not.toContain("workspace-navigation");
  expect(markup).not.toContain("/web/v1/");
});
