import { fireEvent, render, screen } from "@testing-library/react";
import { createRef } from "react";
import { describe, expect, it, vi } from "vitest";

import { Button } from "./Button";

describe("Button", () => {
  it("forwards its ref to the native button", () => {
    const ref = createRef<HTMLButtonElement>();

    render(<Button ref={ref}>Focusable button</Button>);

    expect(ref.current).toBe(screen.getByRole("button", { name: "Focusable button" }));
  });

  it("does not call its handler when disabled", () => {
    const onClick = vi.fn();

    render(
      <Button disabled onClick={onClick}>
        Test button
      </Button>,
    );

    fireEvent.click(screen.getByRole("button", { name: "Test button" }));

    expect(onClick).not.toHaveBeenCalled();
  });
});
