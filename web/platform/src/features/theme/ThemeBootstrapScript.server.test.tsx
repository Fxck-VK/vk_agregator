// @vitest-environment node
import { renderToStaticMarkup } from "react-dom/server";
import { expect, it } from "vitest";

import { ThemeBootstrapScript } from "./ThemeBootstrapScript";

it("emits an executable theme bootstrap with the CSP nonce during server rendering", () => {
  const markup = renderToStaticMarkup(<ThemeBootstrapScript nonce="theme-bootstrap-test" />);
  expect(markup).toContain('type="text/javascript"');
  expect(markup).toContain('nonce="theme-bootstrap-test"');
  expect(markup).toContain('localStorage.getItem("neirohub.theme")');
});
