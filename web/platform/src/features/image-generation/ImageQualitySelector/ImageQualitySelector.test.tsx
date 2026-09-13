import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ImageQualitySelector } from "./ImageQualitySelector";

const gptImage25QualityOptions = [
  "1K-low",
  "1K-medium",
  "1K-high",
  "1K-xhigh",
  "1K-max",
  "2K-low",
  "2K-medium",
  "2K-high",
  "2K-xhigh",
  "2K-max",
  "4K-low",
  "4K-medium",
  "4K-high",
  "4K-xhigh",
  "4K-max",
];

describe("ImageQualitySelector", () => {
  afterEach(cleanup);

  it("opens the resolution menu and selects a quality", () => {
    const onChange = vi.fn();
    render(
      <ImageQualitySelector
        disabled={false}
        label="Разрешение"
        onChange={onChange}
        options={["1K", "2K", "4K"]}
        value="1K"
      />,
    );

    const trigger = screen.getByRole("button", { name: "Разрешение: 1K" });
    expect(trigger.querySelector(
      'img[src="/assets/icons/ui/resolution-white.svg"]',
    )).toBeInTheDocument();
    fireEvent.click(trigger);

    expect(screen.getByRole("dialog", { name: "Разрешение" })).toBeVisible();
    expect(screen.getByRole("radio", { name: "1K" })).toHaveAttribute("aria-checked", "true");
    fireEvent.click(screen.getByRole("radio", { name: "4K" }));

    expect(onChange).toHaveBeenCalledWith("4K");
    expect(screen.queryByRole("dialog", { name: "Разрешение" })).not.toBeInTheDocument();
  });

  it("closes on an outside press and Escape", () => {
    render(
      <div>
        <ImageQualitySelector
          disabled={false}
          label="Разрешение"
          onChange={vi.fn()}
          options={["1K", "2K"]}
          value="1K"
        />
        <button type="button">Outside</button>
      </div>,
    );

    const trigger = screen.getByRole("button", { name: "Разрешение: 1K" });
    fireEvent.click(trigger);
    fireEvent.mouseDown(screen.getByRole("button", { name: "Outside" }));
    expect(screen.queryByRole("dialog", { name: "Разрешение" })).not.toBeInTheDocument();

    fireEvent.click(trigger);
    fireEvent.keyDown(document, { key: "Escape" });
    expect(screen.queryByRole("dialog", { name: "Разрешение" })).not.toBeInTheDocument();
    expect(trigger).toHaveFocus();
  });

  it("disables the trigger when there are no quality options", () => {
    render(
      <ImageQualitySelector
        disabled={false}
        label="Разрешение"
        onChange={vi.fn()}
        options={[]}
        value="1K"
      />,
    );

    expect(screen.getByRole("button", { name: "Разрешение: 1K" })).toBeDisabled();
  });

  it("shows composite GPT quality labels without changing selected values", () => {
    const onChange = vi.fn();
    render(
      <ImageQualitySelector
        disabled={false}
        label="Разрешение"
        onChange={onChange}
        options={gptImage25QualityOptions}
        value="1K-medium"
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Разрешение: 1K·Среднее" }));
    expect(screen.getAllByRole("radio")).toHaveLength(15);
    expect(screen.queryByRole("radio", { name: "1K-medium" })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("radio", { name: "4K·Максимум" }));

    expect(onChange).toHaveBeenCalledWith("4K-max");
  });
});
