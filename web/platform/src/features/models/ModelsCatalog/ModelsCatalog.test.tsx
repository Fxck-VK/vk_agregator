import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("@/lib/web-api/browser", () => ({
  webBrowserFetch: vi.fn(),
}));

import { ru } from "@/i18n/ru";
import { webBrowserFetch } from "@/lib/web-api/browser";

import { ModelsCatalog } from "./ModelsCatalog";

const modelsResponse = {
  items: [
    {
      id: "nano banana/2&preview",
      name: "Nano Banana",
      quality_options: ["1K", "2K"],
      default_quality: "1K",
      supports_reference_image: true,
      max_reference_images: 1,
    },
    {
      id: "other-model",
      name: "Other Model",
      quality_options: ["4K"],
      default_quality: "4K",
      supports_reference_image: false,
      max_reference_images: 0,
    },
    {
      id: "reference-2k",
      name: "Reference 2K",
      quality_options: ["2K"],
      default_quality: "2K",
      supports_reference_image: true,
      max_reference_images: 2,
    },
  ],
};

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe("ModelsCatalog", () => {
  it("loads catalog data, exposes truthful DTO card facts, and links to the selected generator", async () => {
    vi.mocked(webBrowserFetch).mockResolvedValue(Response.json(modelsResponse));
    render(<ModelsCatalog />);

    expect(webBrowserFetch).toHaveBeenCalledWith("/web/v1/image-models");
    expect(await screen.findByRole("link", { name: `${ru.modelsCatalog.openGeneratorLabel}: Nano Banana` })).toHaveAttribute(
      "href",
      "/app/image?model=nano%20banana%2F2%26preview",
    );
    const nanoCard = screen.getByText("Nano Banana").closest("article")!;
    const otherCard = screen.getByText("Other Model").closest("article")!;
    expect(within(nanoCard).getByText(ru.modelsCatalog.imageTypeLabel)).toBeInTheDocument();
    expect(within(nanoCard).getByText("1K")).toBeInTheDocument();
    expect(within(nanoCard).getByText("2K")).toBeInTheDocument();
    expect(within(nanoCard).getByText(ru.modelsCatalog.referenceSupportedLabel)).toBeInTheDocument();
    expect(within(otherCard).getByText(ru.modelsCatalog.referenceUnsupportedLabel)).toBeInTheDocument();
    expect(screen.queryByText(/provider|price|description/i)).not.toBeInTheDocument();

    fireEvent.change(screen.getByRole("searchbox", { name: ru.modelsCatalog.searchLabel }), {
      target: { value: "banana" },
    });
    expect(screen.queryByText("Other Model")).not.toBeInTheDocument();
  });

  it("shows a loading status while the catalog request is pending", () => {
    vi.mocked(webBrowserFetch).mockReturnValue(new Promise<Response>(() => {}));
    render(<ModelsCatalog />);

    expect(screen.getByRole("status")).toHaveTextContent(ru.modelsCatalog.loading);
  });

  it.each([
    ["a failed response", () => Promise.resolve(new Response(null, { status: 500 }))],
    ["an invalid response", () => Promise.resolve(Response.json({ items: [{ id: "missing required fields" }] }))],
    ["a rejected request", () => Promise.reject(new Error("untrusted backend detail"))],
  ])("shows a neutral alert after %s", async (_caseName, request) => {
    vi.mocked(webBrowserFetch).mockImplementationOnce(request);
    render(<ModelsCatalog />);

    expect(await screen.findByRole("alert")).toHaveTextContent(ru.modelsCatalog.loadFailure);
    expect(screen.queryByText("untrusted backend detail")).not.toBeInTheDocument();
  });

  it("distinguishes a valid empty catalog from a load failure", async () => {
    vi.mocked(webBrowserFetch).mockResolvedValue(Response.json({ items: [] }));
    render(<ModelsCatalog />);

    expect(await screen.findByText(ru.modelsCatalog.empty)).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("combines reference-image and quality filters", async () => {
    vi.mocked(webBrowserFetch).mockResolvedValue(Response.json(modelsResponse));
    render(<ModelsCatalog />);

    await screen.findByText("Nano Banana");
    fireEvent.click(screen.getByRole("checkbox", { name: ru.modelsCatalog.referenceFilterLabel }));
    fireEvent.change(screen.getByRole("combobox", { name: ru.modelsCatalog.qualityFilterLabel }), {
      target: { value: "2K" },
    });

    expect(screen.getByText("Nano Banana")).toBeInTheDocument();
    expect(screen.getByText("Reference 2K")).toBeInTheDocument();
    expect(screen.queryByText("Other Model")).not.toBeInTheDocument();
  });
});
