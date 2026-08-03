import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ru } from "@/i18n/ru";

import { ImageGenerationConfirmation } from "./ImageGenerationConfirmation";

const preparation = {
  job: {
    id: "d7c979f5-24e5-4f88-924b-a592d6e5a906",
    status: "prepared" as const,
    prompt: "night city after rain",
    model_id: "nano-banana-2",
    model_name: "Nano Banana 2",
    image_quality: "2K",
    cost_estimate: 60,
    created_at: "2026-08-01T12:00:00Z",
    updated_at: "2026-08-01T12:00:00Z",
  },
  balance: 104,
  can_afford: true,
};

describe("ImageGenerationConfirmation", () => {
  afterEach(() => {
    cleanup();
  });

  it("renders the server-provided estimate and requires an explicit activation", () => {
    const onConfirm = vi.fn();
    render(
      <ImageGenerationConfirmation
        errorMessage={null}
        isActivating={false}
        onConfirm={onConfirm}
        preparation={preparation}
      />,
    );

    expect(screen.getByRole("heading", { name: ru.imageGeneration.confirmationTitle })).toBeInTheDocument();
    expect(screen.getByText(ru.imageGeneration.costLabel)).toBeInTheDocument();
    expect(screen.getByText("60 ★")).toBeInTheDocument();
    expect(screen.getByText("104 ★")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: new RegExp(ru.imageGeneration.confirm) }));
    expect(onConfirm).toHaveBeenCalledTimes(1);
    expect(screen.queryByText(/provider|pricing_snapshot|model_code/i)).not.toBeInTheDocument();
  });

  it("keeps the quote visible when activation reports a neutral error", () => {
    render(
      <ImageGenerationConfirmation
        errorMessage={ru.imageGeneration.insufficientBalance}
        isActivating={false}
        onConfirm={vi.fn()}
        preparation={preparation}
      />,
    );

    expect(screen.getByRole("alert")).toHaveTextContent(ru.imageGeneration.insufficientBalance);
    expect(screen.getByText("60 ★")).toBeInTheDocument();
  });
});
