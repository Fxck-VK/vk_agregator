import { fireEvent, render, screen } from "@testing-library/react";
import { createRef } from "react";
import { describe, expect, it, vi } from "vitest";

import { ModalCloseButton } from "./ModalCloseButton";

describe("ModalCloseButton", () => {
  it("renders an accessible close button and forwards native button props", () => {
    const onClick = vi.fn();

    render(<ModalCloseButton aria-label="Закрыть" onClick={onClick} />);
    const button = screen.getByRole("button", { name: "Закрыть" });

    expect(button).toHaveAttribute("type", "button");
    expect(button.querySelector('[aria-hidden="true"]')).not.toBeNull();

    fireEvent.click(button);
    expect(onClick).toHaveBeenCalledOnce();
  });

  it("forwards its ref and accepts a placement class", () => {
    const ref = createRef<HTMLButtonElement>();

    render(
      <ModalCloseButton
        aria-label="Закрыть"
        className="dialog-placement"
        ref={ref}
      />,
    );

    const button = screen.getByRole("button", { name: "Закрыть" });
    expect(ref.current).toBe(button);
    expect(button).toHaveClass("dialog-placement");
  });
});
