import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { LegalPaymentNotice } from "./LegalLinks";
afterEach(cleanup);
describe("payment legal notice", () => {
  it("preserves the default top-up notice", () => {
    render(<LegalPaymentNotice/>);
    expect(screen.getByText(/Покупка пакета не включает автопродление/)).toBeInTheDocument();
  });
  it("allows a subscription-specific notice while keeping legal links", () => {
    render(<LegalPaymentNotice notice="Продление выбирается отдельно."/>);
    expect(screen.getByText("Продление выбирается отдельно.")).toBeInTheDocument();
    expect(screen.queryByText(/Покупка пакета не включает автопродление/)).not.toBeInTheDocument();
    expect(screen.getAllByRole("link")).toHaveLength(2);
  });
});
