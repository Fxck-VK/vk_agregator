import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { WorkspaceFileDropZone, useWorkspaceFileDrop } from "./WorkspaceFileDropZone";

afterEach(cleanup);

function Receiver({ onFiles, enabled = true }: { onFiles: (files: File[]) => void; enabled?: boolean }) {
  useWorkspaceFileDrop(onFiles, enabled);
  return <textarea aria-label="Сообщение" />;
}

const transfer = (files: File[] = []) => ({ dataTransfer: { types: ["Files"], files, dropEffect: "none" } });

function setup(enabled = true) {
  const onFiles = vi.fn();
  const view = render(<><aside>Боковая панель</aside><WorkspaceFileDropZone><Receiver enabled={enabled} onFiles={onFiles} /></WorkspaceFileDropZone></>);
  const workspace = view.container.querySelector('[data-ui="workspace-file-drop"]')!;
  return { ...view, onFiles, workspace };
}

describe("WorkspaceFileDropZone", () => {
  it("activates only over the workspace and delivers files without submitting", () => {
    const { onFiles, workspace } = setup();
    fireEvent.dragEnter(screen.getByRole("complementary"), transfer());
    expect(screen.queryByRole("status")).toBeNull();
    fireEvent.dragEnter(workspace, transfer());
    expect(screen.getByRole("status")).toHaveTextContent("Отпустите файл, чтобы прикрепить");
    const file = new File(["content"], "note.txt", { type: "text/plain" });
    fireEvent.drop(screen.getByRole("textbox"), transfer([file]));
    expect(onFiles).toHaveBeenCalledExactlyOnceWith([file]);
    expect(screen.queryByRole("status")).toBeNull();
  });

  it("does not flicker between children and clears on leaving or cancellation", () => {
    const { workspace } = setup();
    fireEvent.dragEnter(workspace, transfer());
    fireEvent.dragEnter(screen.getByRole("textbox"), transfer());
    fireEvent.dragLeave(workspace, transfer());
    expect(screen.getByRole("status")).toBeVisible();
    fireEvent.dragLeave(screen.getByRole("textbox"), transfer());
    expect(screen.queryByRole("status")).toBeNull();
    fireEvent.dragEnter(workspace, transfer());
    fireEvent.keyDown(document, { key: "Escape" });
    expect(screen.queryByRole("status")).toBeNull();
    fireEvent.dragEnter(workspace, transfer());
    fireEvent.dragOver(screen.getByRole("complementary"), transfer());
    expect(screen.queryByRole("status")).toBeNull();
  });

  it("ignores text drags and disables file dropping without a receiver", () => {
    const { workspace, onFiles } = setup(false);
    fireEvent.dragEnter(workspace, { dataTransfer: { types: ["text/plain"] } });
    expect(screen.queryByRole("status")).toBeNull();
    fireEvent.dragEnter(workspace, transfer());
    fireEvent.drop(workspace, transfer([new File(["x"], "file.txt")]));
    expect(screen.queryByRole("status")).toBeNull();
    expect(onFiles).not.toHaveBeenCalled();
  });
});
