import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { useState } from "react";
import { afterEach, expect, it, vi } from "vitest";
import { VerificationCodeField } from "./VerificationCodeField";

afterEach(() => { cleanup(); vi.useRealTimers(); });
function ControlledCode({ onComplete = vi.fn(), disabled = false }: { onComplete?: (value: string) => void; disabled?: boolean }) {
  const [value, setValue] = useState("");
  return <VerificationCodeField id="test-code" value={value} onChange={setValue} onComplete={onComplete} disabled={disabled} />;
}

it("accepts a whole pasted code and completes it once, without resubmitting on rerender", () => {
  vi.useFakeTimers();
  const onComplete = vi.fn();
  const { rerender } = render(<ControlledCode onComplete={onComplete} />);
  const fields = screen.getAllByRole("textbox");
  expect(fields).toHaveLength(6);
  expect(fields[0]).toHaveAttribute("autocomplete", "one-time-code");
  expect(fields[0]).toHaveAttribute("inputmode", "numeric");
  fireEvent.paste(fields[2], { clipboardData: { getData: () => "123456" } });
  expect(fields.map(field => (field as HTMLInputElement).value)).toEqual(["1", "2", "3", "4", "5", "6"]);
  act(() => { vi.advanceTimersByTime(1000); });
  expect(onComplete).toHaveBeenCalledExactlyOnceWith("123456");
  rerender(<ControlledCode onComplete={onComplete} />);
  act(() => { vi.advanceTimersByTime(2000); });
  expect(onComplete).toHaveBeenCalledTimes(1);
});

it("moves through cells with typing and arrows, and preserves holes after Backspace", () => {
  render(<ControlledCode />);
  const fields = screen.getAllByRole("textbox");
  fireEvent.change(fields[0], { target: { value: "1" } });
  expect(fields[1]).toHaveFocus();
  fireEvent.change(fields[1], { target: { value: "2" } });
  fireEvent.change(fields[2], { target: { value: "3" } });
  fireEvent.keyDown(fields[2], { key: "ArrowLeft" });
  expect(fields[1]).toHaveFocus();
  fireEvent.keyDown(fields[1], { key: "Backspace" });
  expect(fields[0]).toHaveFocus();
  expect(fields.map(field => (field as HTMLInputElement).value)).toEqual(["1", "", "3", "", "", ""]);
  fireEvent.change(fields[1], { target: { value: "9" } });
  expect((fields[2] as HTMLInputElement).value).toBe("3");
});

it("accepts a mobile autofill code through the first input and suppresses pending submissions", () => {
  vi.useFakeTimers();
  const onComplete = vi.fn();
  const { rerender } = render(<ControlledCode onComplete={onComplete} />);
  fireEvent.change(screen.getAllByRole("textbox")[0], { target: { value: "654321" } });
  rerender(<ControlledCode onComplete={onComplete} disabled />);
  expect(screen.getAllByRole("textbox").every(field => (field as HTMLInputElement).disabled)).toBe(true);
  act(() => { vi.advanceTimersByTime(1500); });
  expect(onComplete).not.toHaveBeenCalled();
  rerender(<ControlledCode onComplete={onComplete} />);
  act(() => { vi.advanceTimersByTime(1500); });
  expect(onComplete).not.toHaveBeenCalled();
});

it("does not complete non-numeric input or an incomplete code", () => {
  vi.useFakeTimers();
  const onComplete = vi.fn();
  render(<ControlledCode onComplete={onComplete} />);
  const fields = screen.getAllByRole("textbox");
  fireEvent.change(fields[0], { target: { value: "a" } });
  expect(fields[0]).toHaveValue("");
  fireEvent.paste(fields[0], { clipboardData: { getData: () => "123" } });
  act(() => { vi.advanceTimersByTime(1500); });
  expect(onComplete).not.toHaveBeenCalled();
});
