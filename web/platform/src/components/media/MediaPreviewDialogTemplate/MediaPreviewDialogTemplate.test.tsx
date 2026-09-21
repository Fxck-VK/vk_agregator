/* eslint-disable @next/next/no-img-element */

import { useState } from "react";
import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { MediaPreviewDialogTemplate } from "./MediaPreviewDialogTemplate";

type TestItem = {
  disabled?: boolean;
  id: string;
  label: string;
};

const items: readonly TestItem[] = [
  { id: "one", label: "Первый" },
  { id: "two", label: "Второй" },
  { id: "three", label: "Третий" },
];

function TemplateHarness({
  initialSelectedIndex = 0,
  isItemSelectable,
  onClose = vi.fn(),
  onShare = vi.fn(),
  previewActions = false,
  previewFooter,
  mediaOnly = false,
  testItems = items,
}: Readonly<{
  initialSelectedIndex?: number;
  isItemSelectable?: (item: TestItem) => boolean;
  onClose?: () => void;
  onShare?: () => void;
  previewActions?: boolean;
  previewFooter?: boolean;
  mediaOnly?: boolean;
  testItems?: readonly TestItem[];
}>) {
  const [selectedIndex, setSelectedIndex] = useState(initialSelectedIndex);

  return (
    <MediaPreviewDialogTemplate
      ariaLabel="Просмотр примера"
      backdropTestId="template-backdrop"
      closeLabel="Закрыть"
      getActions={previewActions ? (item: TestItem) => ({
        download: {
          download: `${item.id}.png`,
          href: `/download/${item.id}`,
          label: "Скачать",
        },
        primary: {
          href: `/recreate/${item.id}`,
          label: "Пересоздать",
        },
        share: {
          label: "Поделиться",
          onClick: onShare,
        },
      }) : undefined}
      getItemKey={(item) => item.id}
      getPreviewDimensions={() => ({ height: 1, width: 1 })}
      getThumbnailLabel={(item) => `Показать ${item.label}`}
      infoPanel={mediaOnly ? undefined : (item) => <p>Панель: {item.label}</p>}
      infoPanelTestId="template-info-panel"
      isItemSelectable={isItemSelectable}
      items={testItems}
      nextLabel="Следующий"
      onClose={onClose}
      onSelect={setSelectedIndex}
      previousLabel="Предыдущий"
      renderPreview={(item, classes) => (
        <div className={classes.previewMedia}>Превью: {item.label}</div>
      )}
      renderPreviewFooter={previewFooter ? (item) => (
        <button type="button">Инструменты: {item.label}</button>
      ) : undefined}
      renderThumbnail={(item, classes) => (
        <span className={classes.thumbnailMedia}>Миниатюра: {item.label}</span>
      )}
      selectedIndex={selectedIndex}
      testIdPrefix="template"
      thumbnailRailLabel="Примеры"
    />
  );
}

describe("MediaPreviewDialogTemplate", () => {
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it("omits the information panel in media-only mode while retaining gallery navigation", () => {
    render(<TemplateHarness mediaOnly />);
    expect(screen.queryByRole("complementary")).not.toBeInTheDocument();
    expect(screen.getByRole("dialog")).toHaveAttribute("data-media-only", "true");
    fireEvent.click(screen.getByRole("button", { name: "Следующий" }));
    expect(screen.getByText("Превью: Второй")).toBeInTheDocument();
    fireEvent.keyDown(document, { key: "ArrowLeft" });
    expect(screen.getByText("Превью: Первый")).toBeInTheDocument();
  });

  it("renders the selected preview and consumer-provided information panel", () => {
    render(<TemplateHarness />);

    const dialog = screen.getByRole("dialog", { name: "Просмотр примера" });
    expect(within(dialog).getByText("Превью: Первый")).toBeInTheDocument();
    expect(within(dialog).getByText("Панель: Первый")).toBeInTheDocument();
    expect(within(dialog).getAllByTestId("template-thumbnail")).toHaveLength(3);
  });

  it("uses the loaded image dimensions instead of inaccurate consumer metadata", () => {
    render(
      <MediaPreviewDialogTemplate
        ariaLabel="Просмотр изображения"
        closeLabel="Закрыть"
        getItemKey={(item: TestItem) => item.id}
        getPreviewDimensions={() => ({ height: 1024, width: 1024 })}
        getThumbnailLabel={(item) => `Показать ${item.label}`}
        infoPanel={() => null}
        items={[items[0]!]}
        nextLabel="Следующий"
        onClose={vi.fn()}
        onSelect={vi.fn()}
        previousLabel="Предыдущий"
        renderPreview={(item, classes) => (
          <img alt={item.label} className={classes.previewMedia} src="/portrait.png" />
        )}
        renderThumbnail={() => null}
        selectedIndex={0}
        testIdPrefix="intrinsic-image"
        thumbnailRailLabel="Изображения"
      />,
    );

    const image = screen.getByRole("img", { name: "Первый" });
    const surface = screen.getByTestId("intrinsic-image-media-surface");
    expect(surface.style.aspectRatio).toBe("1024 / 1024");

    Object.defineProperties(image, {
      naturalHeight: { configurable: true, value: 1536 },
      naturalWidth: { configurable: true, value: 1024 },
    });
    fireEvent.load(image);

    expect(surface.style.aspectRatio).toBe("1024 / 1536");
  });

  it("uses the loaded video dimensions instead of inaccurate consumer metadata", () => {
    render(
      <MediaPreviewDialogTemplate
        ariaLabel="Просмотр видео"
        closeLabel="Закрыть"
        getItemKey={(item: TestItem) => item.id}
        getPreviewDimensions={() => ({ height: 1024, width: 1024 })}
        getThumbnailLabel={(item) => `Показать ${item.label}`}
        infoPanel={() => null}
        items={[items[0]!]}
        nextLabel="Следующий"
        onClose={vi.fn()}
        onSelect={vi.fn()}
        previousLabel="Предыдущий"
        renderPreview={(item, classes) => (
          <video aria-label={item.label} className={classes.previewMedia} />
        )}
        renderThumbnail={() => null}
        selectedIndex={0}
        testIdPrefix="intrinsic-video"
        thumbnailRailLabel="Видео"
      />,
    );

    const video = screen.getByLabelText("Первый");
    const surface = screen.getByTestId("intrinsic-video-media-surface");
    Object.defineProperties(video, {
      videoHeight: { configurable: true, value: 1080 },
      videoWidth: { configurable: true, value: 1920 },
    });
    fireEvent.loadedMetadata(video);

    expect(surface.style.aspectRatio).toBe("1920 / 1080");
  });

  it("keeps initial and arrow-navigation focus on the dialog surface", () => {
    render(<TemplateHarness />);

    const dialog = screen.getByRole("dialog", { name: "Просмотр примера" });
    expect(dialog).toHaveFocus();

    fireEvent.keyDown(document, { key: "ArrowRight" });
    expect(dialog).toHaveFocus();
    expect(screen.getByRole("button", { name: "Закрыть" })).not.toHaveFocus();
  });

  it("selects thumbnails and wraps previous and next navigation", () => {
    render(<TemplateHarness />);

    const dialog = screen.getByRole("dialog", { name: "Просмотр примера" });
    const thumbnails = within(dialog).getAllByTestId("template-thumbnail");
    const previousIcon = within(dialog).getByRole("button", { name: "Предыдущий" }).querySelector("img");
    const nextIcon = within(dialog).getByRole("button", { name: "Следующий" }).querySelector("img");

    for (const icon of [previousIcon, nextIcon]) {
      expect(icon).toHaveAttribute("src", "/assets/icons/ui/faq-arrow.svg");
      expect(icon).toHaveAttribute("width", "14");
      expect(icon).toHaveAttribute("height", "8");
    }

    fireEvent.click(thumbnails[1]);
    expect(thumbnails[1]).toHaveAttribute("aria-current", "true");
    expect(within(dialog).getByText("Превью: Второй")).toBeInTheDocument();

    fireEvent.click(within(dialog).getByRole("button", { name: "Предыдущий" }));
    expect(thumbnails[0]).toHaveAttribute("aria-current", "true");

    fireEvent.click(within(dialog).getByRole("button", { name: "Предыдущий" }));
    expect(thumbnails[2]).toHaveAttribute("aria-current", "true");

    fireEvent.click(within(dialog).getByRole("button", { name: "Следующий" }));
    expect(thumbnails[0]).toHaveAttribute("aria-current", "true");
  });

  it("handles document arrow navigation but ignores editable controls", () => {
    render(<TemplateHarness />);

    const dialog = screen.getByRole("dialog", { name: "Просмотр примера" });
    const thumbnails = within(dialog).getAllByTestId("template-thumbnail");

    fireEvent.keyDown(document, { key: "ArrowRight" });
    expect(thumbnails[1]).toHaveAttribute("aria-current", "true");

    const input = document.createElement("input");
    dialog.append(input);
    fireEvent.keyDown(input, { key: "ArrowRight" });
    expect(thumbnails[1]).toHaveAttribute("aria-current", "true");
  });

  it("normalizes an out-of-range selected index before rendering and navigating", () => {
    render(<TemplateHarness initialSelectedIndex={99} />);

    const dialog = screen.getByRole("dialog", { name: "Просмотр примера" });
    const thumbnails = within(dialog).getAllByTestId("template-thumbnail");

    expect(thumbnails[0]).toHaveAttribute("aria-current", "true");
    expect(within(dialog).getByText("Превью: Первый")).toBeInTheDocument();

    fireEvent.click(within(dialog).getByRole("button", { name: "Следующий" }));
    expect(thumbnails[1]).toHaveAttribute("aria-current", "true");
    expect(within(dialog).getByText("Превью: Второй")).toBeInTheDocument();
  });

  it("keeps disabled thumbnails visible and skips them during navigation", () => {
    const testItems = items.map((item, index) => ({ ...item, disabled: index === 1 }));
    render(
      <TemplateHarness
        isItemSelectable={(item) => !item.disabled}
        testItems={testItems}
      />,
    );

    const dialog = screen.getByRole("dialog", { name: "Просмотр примера" });
    const thumbnails = within(dialog).getAllByTestId("template-thumbnail");
    expect(thumbnails[1]).toBeDisabled();

    fireEvent.click(within(dialog).getByRole("button", { name: "Следующий" }));
    expect(thumbnails[2]).toHaveAttribute("aria-current", "true");
  });

  it("does not render a dialog when every item is non-selectable", () => {
    render(<TemplateHarness isItemSelectable={() => false} />);

    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("renders consumer footer content and exposes shared test hooks", () => {
    render(<TemplateHarness previewFooter />);

    expect(screen.getByText("Инструменты: Первый")).toBeInTheDocument();
    expect(screen.getByTestId("template-backdrop")).toHaveAttribute("data-state", "open");
    expect(screen.getByTestId("template-info-panel")).toHaveAttribute(
      "data-scroll-area-viewport",
      "true",
    );
  });

  it("renders the shared recreate, download and share actions", () => {
    const onShare = vi.fn();
    render(<TemplateHarness onShare={onShare} previewActions />);

    const recreate = screen.getByRole("link", { name: "Пересоздать" });
    const download = screen.getByRole("link", { name: "Скачать" });
    const share = screen.getByRole("button", { name: "Поделиться" });

    expect(recreate).toHaveAttribute("href", "/ru/recreate/one");
    expect(recreate.querySelector("img")).toHaveAttribute(
      "src",
      "/assets/icons/ui/star-white.svg",
    );
    expect(download).toHaveAttribute("download", "one.png");
    expect(download).toHaveAttribute("href", "/download/one");
    expect(download.querySelector("img")).toHaveAttribute(
      "src",
      "/assets/icons/ui/download-white.svg",
    );
    expect(share.querySelector("img")).toHaveAttribute(
      "src",
      "/assets/icons/ui/repost-white.svg",
    );

    fireEvent.click(share);
    expect(onShare).toHaveBeenCalledOnce();
  });

  it("uses a horizontal thumbnail scrollbar on a narrow viewport", () => {
    vi.stubGlobal("matchMedia", vi.fn().mockReturnValue({
      addEventListener: vi.fn(),
      matches: true,
      removeEventListener: vi.fn(),
    }));

    render(<TemplateHarness />);

    expect(screen.getByRole("navigation", { name: "Примеры" }).closest("[data-orientation]"))
      .toHaveAttribute("data-orientation", "horizontal");
  });

  it("keeps keyboard focus inside the shared dialog", () => {
    render(<TemplateHarness previewFooter />);

    const closeButton = screen.getByRole("button", { name: "Закрыть" });
    const footerButton = screen.getByRole("button", { name: "Инструменты: Первый" });

    closeButton.focus();
    fireEvent.keyDown(document, { key: "Tab", shiftKey: true });
    expect(footerButton).toHaveFocus();

    footerButton.focus();
    fireEvent.keyDown(document, { key: "Tab" });
    expect(closeButton).toHaveFocus();
  });

  it("requests closing through the shared close control", () => {
    const onClose = vi.fn();
    render(<TemplateHarness onClose={onClose} />);

    const dialog = screen.getByRole("dialog", { name: "Просмотр примера" });
    fireEvent.click(screen.getByRole("button", { name: "Закрыть" }));

    const backdrop = dialog.closest("[data-state]")!;
    expect(backdrop).toHaveAttribute("data-state", "closing");
    expect(onClose).not.toHaveBeenCalled();

    fireEvent.animationEnd(backdrop, { animationName: "modalBackdropOut" });
    expect(onClose).toHaveBeenCalledOnce();
  });
});
