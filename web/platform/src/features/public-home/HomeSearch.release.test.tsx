import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { homeEn } from "@/i18n/public/home-en";
import { loadModelCatalog } from "@/features/models/model-catalog-cache";
import { HomeSearch } from "./HomeSearch";

const release = vi.hoisted(() => ({ view: "dev" as "dev" | "prod" }));
vi.mock("@/features/release/ReleaseViewProvider", () => ({ useReleaseView: () => ({ ...release, allowsPath: () => true }) }));
vi.mock("@/features/models/model-catalog-cache", () => ({ loadModelCatalog: vi.fn(), resetModelCatalogCacheForTests: vi.fn() }));
const catalog = (name: string) => ({ items: [{ id: name, name }] }) as Awaited<ReturnType<typeof loadModelCatalog>>;
function deferred<T>() { let resolve!: (value: T) => void, reject!: (reason: Error) => void; const promise = new Promise<T>((yes, no) => { resolve = yes; reject = no; }); return { promise, resolve, reject }; }
beforeEach(() => { release.view = "dev"; vi.mocked(loadModelCatalog).mockReset(); });
afterEach(cleanup);

it("clears old-view model results while reloading and preserves the search query", async () => {
  const next = deferred<Awaited<ReturnType<typeof loadModelCatalog>>>();
  vi.mocked(loadModelCatalog).mockResolvedValueOnce(catalog("Model Dev")).mockReturnValueOnce(next.promise);
  const view = render(<HomeSearch copy={homeEn} onClose={vi.fn()} />);
  await screen.findByRole("link", { name: "Model Dev" });
  fireEvent.change(screen.getByRole("searchbox"), { target: { value: "Model" } });
  release.view = "prod";
  view.rerender(<HomeSearch copy={homeEn} onClose={vi.fn()} />);
  expect(screen.queryByRole("link", { name: "Model Dev" })).not.toBeInTheDocument();
  expect(screen.getByRole("searchbox")).toHaveValue("Model");
  await act(async () => { next.resolve(catalog("Model Prod")); });
  expect(screen.getByRole("link", { name: "Model Prod" })).toBeInTheDocument();
});

it("does not restore old results after a failed PROD refresh or a late DEV response", async () => {
  const old = deferred<Awaited<ReturnType<typeof loadModelCatalog>>>();
  vi.mocked(loadModelCatalog).mockReturnValueOnce(old.promise).mockRejectedValueOnce(new Error("Unavailable"));
  const view = render(<HomeSearch copy={homeEn} onClose={vi.fn()} />);
  release.view = "prod"; view.rerender(<HomeSearch copy={homeEn} onClose={vi.fn()} />);
  await waitFor(() => expect(screen.getByRole("status")).toHaveTextContent(homeEn.controls.modelError));
  await act(async () => { old.resolve(catalog("Model Dev")); });
  expect(screen.queryByRole("link", { name: "Model Dev" })).not.toBeInTheDocument();
  expect(screen.getByRole("status")).toHaveTextContent(homeEn.controls.modelError);
});

it("keeps prior results hidden when switching back before the intervening read settles", async () => {
  const prod = deferred<Awaited<ReturnType<typeof loadModelCatalog>>>();
  const dev = deferred<Awaited<ReturnType<typeof loadModelCatalog>>>();
  vi.mocked(loadModelCatalog).mockResolvedValueOnce(catalog("Model Dev")).mockReturnValueOnce(prod.promise).mockReturnValueOnce(dev.promise);
  const result = render(<HomeSearch copy={homeEn} onClose={vi.fn()} />);
  await screen.findByRole("link", { name: "Model Dev" });
  release.view = "prod"; result.rerender(<HomeSearch copy={homeEn} onClose={vi.fn()} />);
  release.view = "dev"; result.rerender(<HomeSearch copy={homeEn} onClose={vi.fn()} />);
  expect(screen.queryByRole("link", { name: "Model Dev" })).not.toBeInTheDocument();
  await act(async () => { prod.resolve(catalog("Model Prod")); });
  expect(screen.queryByRole("link", { name: "Model Prod" })).not.toBeInTheDocument();
  await act(async () => { dev.resolve(catalog("Model Fresh")); });
  expect(screen.getByRole("link", { name: "Model Fresh" })).toBeInTheDocument();
});
