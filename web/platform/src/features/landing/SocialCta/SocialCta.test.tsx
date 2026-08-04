import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { SocialCta } from "./SocialCta";

describe("SocialCta", () => {
  it("does not invent unconfirmed social URLs", () => {
    render(<SocialCta links={[]} />);
    expect(screen.getByText(/официальные каналы появятся/i)).toBeInTheDocument();
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
  });
});
