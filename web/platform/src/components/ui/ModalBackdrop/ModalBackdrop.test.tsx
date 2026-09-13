import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ModalBackdrop } from "./ModalBackdrop";

describe("ModalBackdrop", () => {
  afterEach(() => {
    cleanup();
    document.querySelector("[data-modal-test-scroll-region]")?.remove();
    vi.unstubAllGlobals();
  });

  it("renders directly under the document body to escape ancestor stacking contexts", () => {
    render(
      <section data-testid="stacking-context">
        <ModalBackdrop onClose={vi.fn()} testId="ported-modal-backdrop">
          <div>Содержимое окна</div>
        </ModalBackdrop>
      </section>,
    );

    const backdrop = screen.getByTestId("ported-modal-backdrop");
    expect(backdrop.parentElement).toBe(document.body);
    expect(screen.getByTestId("stacking-context")).not.toContainElement(backdrop);
  });

  it("extends the application-only hover scope into a portalled modal", () => {
    render(
      <div data-app-shell="">
        <ModalBackdrop onClose={vi.fn()} testId="app-modal-backdrop">
          <button type="button">Действие</button>
        </ModalBackdrop>
      </div>,
    );

  });

  it("closes only when the shared background itself is pressed", () => {
    const onClose = vi.fn();
    render(
      <ModalBackdrop onClose={onClose} testId="shared-modal-backdrop">
        <section>Содержимое окна</section>
      </ModalBackdrop>,
    );

    fireEvent.mouseDown(screen.getByText("Содержимое окна"));
    expect(onClose).not.toHaveBeenCalled();

    const backdrop = screen.getByTestId("shared-modal-backdrop");
    fireEvent.mouseDown(backdrop);
    expect(backdrop).toHaveAttribute("data-state", "closing");
    expect(onClose).not.toHaveBeenCalled();

    fireEvent.animationEnd(backdrop, { animationName: "modalBackdropOut" });
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("locks page scrolling while mounted and restores it after unmount", () => {
    const scrollRegion = document.createElement("div");
    scrollRegion.dataset.testid = "workspace-scroll-region";
    scrollRegion.dataset.modalTestScrollRegion = "true";
    scrollRegion.style.overflowY = "scroll";
    document.body.append(scrollRegion);

    const view = render(
      <ModalBackdrop onClose={vi.fn()}>
        <section>Содержимое окна</section>
      </ModalBackdrop>,
    );

    expect(document.body.style.overflow).toBe("hidden");
    expect(scrollRegion.style.overflowY).toBe("hidden");

    view.unmount();
    expect(document.body.style.overflow).toBe("");
    expect(scrollRegion.style.overflowY).toBe("scroll");
  });

  it("finishes its exit animation before restoring scrolling and notifying its owner", () => {
    const onClose = vi.fn(() => {
      expect(document.body.style.overflow).toBe("");
    });
    render(
      <ModalBackdrop onClose={onClose} testId="animated-modal-backdrop">
        <section>Содержимое окна</section>
      </ModalBackdrop>,
    );

    fireEvent.keyDown(window, { key: "Escape" });

    expect(onClose).not.toHaveBeenCalled();
    expect(document.body.style.overflow).toBe("hidden");

    fireEvent.animationEnd(screen.getByTestId("animated-modal-backdrop"), { animationName: "modalBackdropOut" });
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("closes immediately when reduced motion is requested", () => {
    const onClose = vi.fn();
    vi.stubGlobal("matchMedia", vi.fn(() => ({ matches: true } as MediaQueryList)));
    render(
      <ModalBackdrop onClose={onClose} testId="reduced-motion-backdrop">
        <section>Содержимое окна</section>
      </ModalBackdrop>,
    );

    fireEvent.mouseDown(screen.getByTestId("reduced-motion-backdrop"));

    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("lets modal content request the same animated close sequence", () => {
    const onClose = vi.fn();
    render(
      <ModalBackdrop onClose={onClose} testId="content-close-backdrop">
        {(requestClose) => <button onClick={requestClose} type="button">Закрыть окно</button>}
      </ModalBackdrop>,
    );

    fireEvent.click(screen.getByRole("button", { name: "Закрыть окно" }));
    const backdrop = screen.getByTestId("content-close-backdrop");
    expect(backdrop).toHaveAttribute("data-state", "closing");
    expect(onClose).not.toHaveBeenCalled();

    fireEvent.animationEnd(backdrop, { animationName: "modalBackdropOut" });
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("can keep a pending modal open when its background is pressed", () => {
    const onClose = vi.fn();
    render(
      <ModalBackdrop closeOnBackdropClick={false} closeOnEscape={false} onClose={onClose} testId="pending-modal-backdrop">
        <section>Выполняется операция</section>
      </ModalBackdrop>,
    );

    fireEvent.mouseDown(screen.getByTestId("pending-modal-backdrop"));
    fireEvent.keyDown(window, { key: "Escape" });
    expect(onClose).not.toHaveBeenCalled();
  });
});
