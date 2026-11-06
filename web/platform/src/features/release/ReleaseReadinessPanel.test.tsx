import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ReleaseReadinessPanel } from "./ReleaseReadinessPanel";
import { allowsReleasePath, releaseFeatures, releasePages } from "./policy";

const state = vi.hoisted(() => ({ view: "prod" as "prod" | "dev", canSwitch: true, switching: false, error: false, changeView: vi.fn(), push: vi.fn() }));
vi.mock("./ReleaseViewProvider", () => ({ useReleaseView: () => ({ ...state, allowsPath: (path: string) => allowsReleasePath(path, state.view) }) }));
vi.mock("@/i18n/navigation", () => ({ useRouter: () => ({ push: state.push }) }));
beforeEach(() => { state.view = "prod"; state.canSwitch = true; state.switching = false; state.error = false; state.changeView.mockReset().mockResolvedValue(true); state.push.mockReset(); });
afterEach(cleanup);

describe("release readiness inventory", () => {
  it("shows all feature and page records, source readiness details and a keyboard-accessible bounded inventory", () => {
    const { container } = render(<ReleaseReadinessPanel />);
    expect(screen.getByRole("button", { name: "Вид PROD" })).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByRole("button", { name: "Вид DEV" })).toBeEnabled();
    expect(container.querySelectorAll("[data-release-feature]")).toHaveLength(releaseFeatures.length);
    expect(container.querySelectorAll("[data-release-page]")).toHaveLength(releasePages.length);
    expect(screen.getByRole("region", { name: "Готовность функций и страниц" })).toHaveAttribute("tabindex", "0");
    const feature = releaseFeatures.find(item => item.id === "photo-edit")!;
    const row = container.querySelector('[data-release-feature="photo-edit"]')!;
    fireEvent.click(within(row as HTMLElement).getByText(feature.name));
    expect(row).toHaveTextContent(feature.works);
    expect(row).toHaveTextContent(feature.remaining);
    expect(row).toHaveTextContent(feature.verification);
    expect(row).toHaveTextContent("Не подключено");
    expect(row).toHaveTextContent("Только DEV");
  });

  it("switches to DEV before opening an excluded feature even when its parent page is available", async () => {
    let complete!: (result: boolean) => void;
    state.changeView.mockImplementation(() => new Promise<boolean>(resolve => { complete = resolve; }));
    render(<ReleaseReadinessPanel />);
    fireEvent.click(screen.getByRole("button", { name: "Открыть в DEV: Редактирование области фото" }));
    expect(state.changeView).toHaveBeenCalledWith("dev");
    expect(state.push).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "Вид DEV" })).toBeDisabled();
    complete(true);
    await waitFor(() => expect(state.push).toHaveBeenCalledWith("/app/files"));
  });

  it("opens an included feature directly and a draft page only after DEV switching", async () => {
    render(<ReleaseReadinessPanel />);
    fireEvent.click(screen.getByRole("button", { name: "Открыть: Рабочая среда и профиль" }));
    expect(state.push).toHaveBeenCalledWith("/app");
    expect(state.changeView).not.toHaveBeenCalled();
    const page = releasePages.find(item => !allowsReleasePath(item.path, "prod"))!;
    fireEvent.click(screen.getByRole("button", { name: `Открыть в DEV: ${page.title}` }));
    await waitFor(() => expect(state.push).toHaveBeenCalledWith(page.path));
    expect(state.changeView).toHaveBeenCalledWith("dev");
  });

  it("keeps failed switches visible without navigating and disables changes when switching is not allowed", async () => {
    state.changeView.mockResolvedValue(false);
    const view = render(<ReleaseReadinessPanel />);
    fireEvent.click(screen.getByRole("button", { name: "Открыть в DEV: Музыка и Suno Studio" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Не удалось переключить вид");
    expect(state.push).not.toHaveBeenCalled();
    state.canSwitch = false;
    view.rerender(<ReleaseReadinessPanel />);
    expect(screen.getByRole("button", { name: "Вид DEV" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Открыть в DEV: Музыка и Suno Studio" })).toBeDisabled();
  });

  it("switches the selected view without opening a page and reports provider errors", async () => {
    const view = render(<ReleaseReadinessPanel />);
    fireEvent.click(screen.getByRole("button", { name: "Вид DEV" }));
    await waitFor(() => expect(state.changeView).toHaveBeenCalledWith("dev"));
    expect(state.push).not.toHaveBeenCalled();
    state.error = true;
    view.rerender(<ReleaseReadinessPanel />);
    expect(screen.getByRole("alert")).toBeInTheDocument();
  });
});
