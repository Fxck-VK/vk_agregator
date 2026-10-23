import { createRef } from "react";
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { ConfirmDialog } from "./ConfirmDialog";

function dialogProps() {
  return {
    title: "Удалить файл?",
    description: "Восстановить его не получится.",
    cancelLabel: "Отмена",
    confirmLabel: "Удалить",
    pendingLabel: "Удаляем…",
    onCancel: vi.fn(),
    onConfirm: vi.fn(),
  };
}

describe("ConfirmDialog", () => {
  it("names and describes the dialog, focuses cancel and contains keyboard navigation", () => {
    const props = dialogProps();
    render(<ConfirmDialog {...props} lead={<>Это удалит <strong>Идеи для проекта</strong>.</>} />);
    const dialog = screen.getByRole("dialog", { name: props.title });
    expect(dialog).toHaveAccessibleDescription("Это удалит Идеи для проекта. Восстановить его не получится.");
    const cancel = screen.getByRole("button", { name: props.cancelLabel });
    const confirm = screen.getByRole("button", { name: props.confirmLabel });
    expect(cancel).toHaveFocus();
    fireEvent.keyDown(cancel, { key: "Tab", shiftKey: true });
    expect(confirm).toHaveFocus();
    fireEvent.keyDown(confirm, { key: "Tab" });
    expect(cancel).toHaveFocus();
    expect(props.onConfirm).not.toHaveBeenCalled();
    fireEvent.click(confirm);
    expect(props.onConfirm).toHaveBeenCalledTimes(1);
  });

  it("blocks close, repeated confirmation and keyboard escape while pending, then exposes a retryable error", () => {
    const props = dialogProps();
    const view = render(<ConfirmDialog {...props} isPending />);
    const dialog = screen.getByRole("dialog");
    const backdrop = dialog.closest("[data-state]")!;
    expect(dialog).toHaveAttribute("aria-busy", "true");
    expect(dialog).toHaveFocus();
    expect(screen.getByRole("button", { name: props.cancelLabel })).toBeDisabled();
    const confirm = screen.getByRole("button", { name: props.pendingLabel });
    expect(confirm).toBeDisabled();
    fireEvent.click(confirm);
    fireEvent.mouseDown(backdrop);
    fireEvent.keyDown(window, { key: "Escape" });
    expect(backdrop).toHaveAttribute("data-state", "open");
    const tab = new KeyboardEvent("keydown", { key: "Tab", bubbles: true, cancelable: true });
    fireEvent(dialog, tab);
    expect(tab.defaultPrevented).toBe(true);
    expect(props.onCancel).not.toHaveBeenCalled();
    expect(props.onConfirm).not.toHaveBeenCalled();

    view.rerender(<ConfirmDialog {...props} errorMessage="Не удалось удалить. Попробуйте снова." />);
    expect(screen.getByRole("alert")).toHaveTextContent("Попробуйте снова");
    expect(screen.getByRole("button", { name: props.cancelLabel })).toHaveFocus();
    fireEvent.click(screen.getByRole("button", { name: props.confirmLabel }));
    expect(props.onConfirm).toHaveBeenCalledTimes(1);
  });

  it("uses the shared animated close sequence and restores the trigger focus on unmount", () => {
    const props = dialogProps();
    const trigger = createRef<HTMLButtonElement>();
    const view = render(<><button ref={trigger}>Корзина</button><ConfirmDialog {...props} returnFocusRef={trigger} /></>);
    const dialog = screen.getByRole("dialog");
    const backdrop = dialog.closest("[data-state]")!;
    fireEvent.click(screen.getByRole("button", { name: props.cancelLabel }));
    expect(props.onCancel).not.toHaveBeenCalled();
    expect(backdrop).toHaveAttribute("data-state", "closing");
    fireEvent.animationEnd(backdrop, { animationName: "modalBackdropOut" });
    expect(props.onCancel).toHaveBeenCalledTimes(1);
    view.rerender(<button ref={trigger}>Корзина</button>);
    expect(trigger.current).toHaveFocus();
  });

  it("does not steal focus when navigation has moved it outside before unmount", () => {
    const props = dialogProps();
    const trigger = createRef<HTMLButtonElement>();
    const view = render(<><button ref={trigger}>Корзина</button><button>Другой экран</button><ConfirmDialog {...props} returnFocusRef={trigger} /></>);
    screen.getByRole("button", { name: "Другой экран" }).focus();
    view.rerender(<><button ref={trigger}>Корзина</button><button>Другой экран</button></>);
    expect(screen.getByRole("button", { name: "Другой экран" })).toHaveFocus();
  });

  it("lets a composite owner control focus when the deleted row or sidebar is removed", () => {
    const props = dialogProps();
    const view = render(<button>Меню чата</button>);
    screen.getByRole("button", { name: "Меню чата" }).focus();
    view.rerender(<><button>Меню чата</button><ConfirmDialog {...props} restoreFocus={false} /></>);
    expect(screen.getByRole("button", { name: props.cancelLabel })).toHaveFocus();
    view.rerender(<button>Меню чата</button>);
    expect(screen.getByRole("button", { name: "Меню чата" })).not.toHaveFocus();
  });
});
