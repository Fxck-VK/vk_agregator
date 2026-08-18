import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ru } from "@/i18n/ru";

import { SessionRestorationShell } from "./SessionRestorationShell";

afterEach(cleanup);

describe("SessionRestorationShell", () => {
  it("keeps a neutral workspace-shaped frame without private data", () => {
    render(
      <SessionRestorationShell
        isProgressVisible={false}
        isRetryableError={false}
        onRetry={vi.fn()}
      />,
    );

    expect(screen.getByTestId("session-restoration-shell")).toHaveAttribute(
      "aria-busy",
      "true",
    );
    expect(screen.getByTestId("session-restoration-sidebar")).toBeInTheDocument();
    expect(screen.getByTestId("session-restoration-header")).toBeInTheDocument();
    expect(screen.queryByText(/@/)).not.toBeInTheDocument();
    expect(screen.queryByText(/чат/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/★/)).not.toBeInTheDocument();
    expect(screen.queryByRole("progressbar")).not.toBeInTheDocument();
  });

  it("shows the delayed progress line when requested", () => {
    render(
      <SessionRestorationShell
        isProgressVisible
        isRetryableError={false}
        onRetry={vi.fn()}
      />,
    );

    expect(
      screen.getByRole("progressbar", {
        name: ru.workspace.sessionProgressLabel,
      }),
    ).toBeInTheDocument();
  });

  it("shows a compact retry action without destroying the frame", () => {
    const onRetry = vi.fn();

    render(
      <SessionRestorationShell
        isProgressVisible={false}
        isRetryableError
        onRetry={onRetry}
      />,
    );

    expect(screen.getByTestId("session-restoration-shell")).toHaveAttribute(
      "aria-busy",
      "false",
    );
    expect(screen.getByRole("status")).toHaveTextContent(
      ru.workspace.sessionRetryableError,
    );
    fireEvent.click(
      screen.getByRole("button", { name: ru.workspace.sessionRetry }),
    );
    expect(onRetry).toHaveBeenCalledTimes(1);
    expect(screen.getByTestId("session-restoration-sidebar")).toBeInTheDocument();
  });
});
