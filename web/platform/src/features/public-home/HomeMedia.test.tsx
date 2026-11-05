import { render, screen } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import { LocaleProvider } from "@/i18n/LocaleProvider";
import { homeMediaBriefsEn } from "@/i18n/public/home-media-en";
import type { HomeMediaAsset } from "./HomeMedia.assets";
import { HomeMediaPrompt } from "./HomeMedia";

const { assets } = vi.hoisted(() => ({ assets: {} as Record<string, HomeMediaAsset> }));
vi.mock("./HomeMedia.assets", () => ({ homeMediaAssets: assets }));
beforeEach(() => { for (const key of Object.keys(assets)) delete assets[key]; });

it("renders an accessible visible image brief without fetching a reference asset", () => {
  const { container } = render(<LocaleProvider locale="en"><HomeMediaPrompt brief="suiteImage" /></LocaleProvider>);
  expect(screen.getByRole("figure")).toHaveAccessibleName(/Image prompt/);
  expect(screen.getByText(homeMediaBriefsEn.suiteImage.prompt)).toBeVisible();
  expect(screen.getByText(`Format ${homeMediaBriefsEn.suiteImage.format}`)).toBeVisible();
  expect(container.querySelector("img,video,source")).toBeNull();
});

it("replaces the hero prompt with an intrinsically sized eager artwork", () => {
  assets.hero = { src: "/assets/images/home/hero.webp", width: 1600, height: 900,
    srcSet: "/assets/images/home/hero-640.webp 640w, /assets/images/home/hero.webp 1600w",
    sizes: "100vw" };
  const { container } = render(<LocaleProvider locale="en"><HomeMediaPrompt brief="hero" /></LocaleProvider>);
  const figure = screen.getByRole("figure", { name: homeMediaBriefsEn.hero.title });
  const photo = screen.getByRole("img", { name: homeMediaBriefsEn.hero.title });
  expect(photo).toHaveAttribute("src", "/assets/images/home/hero.webp");
  expect(photo).toHaveAttribute("width", "1600");
  expect(photo).toHaveAttribute("height", "900");
  expect(photo).toHaveAttribute("loading", "eager");
  expect(photo).toHaveAttribute("fetchpriority", "high");
  expect(photo.closest('[data-ui="media-image"]')).toHaveAttribute("data-fit", "cover");
  expect(photo).toHaveAttribute("srcset", assets.hero.srcSet);
  expect(photo).toHaveAttribute("sizes", assets.hero.sizes);
  expect(figure).toHaveStyle({ aspectRatio: "1600 / 900" });
  expect(screen.queryByText(homeMediaBriefsEn.hero.prompt)).not.toBeInTheDocument();
  expect(screen.queryByText(`Format ${homeMediaBriefsEn.hero.format}`)).not.toBeInTheDocument();
  expect(container.querySelector("video")).toBeNull();
});

it("renders one accessible lazy suite photo without interface overlays", () => {
  assets.suiteImage = {
    src: "/assets/images/home/suite-image.webp", width: 1600, height: 900,
  };
  const { container } = render(<LocaleProvider locale="en"><HomeMediaPrompt brief="suiteImage" /></LocaleProvider>);
  const figure = screen.getByRole("figure", { name: homeMediaBriefsEn.suiteImage.title });
  expect(figure).toHaveAttribute("data-brief", "suiteImage");
  const photo = screen.getByRole("img", { name: homeMediaBriefsEn.suiteImage.title });
  expect(photo).toHaveAttribute("loading", "lazy");
  expect(photo.closest('[data-ui="media-image"]')).toHaveAttribute("data-fit", "contain");
  expect(container.querySelectorAll("img")).toHaveLength(1);
  expect(container.querySelector("[data-kind]")).toBeNull();
  expect(screen.queryByRole("button")).not.toBeInTheDocument();
  expect(screen.queryByText(homeMediaBriefsEn.suiteImage.prompt)).not.toBeInTheDocument();
});

it("renders only the overview photo without overlay controls", () => {
  assets.overview = {
    src: "/assets/images/home/overview.webp", width: 1600, height: 900,
  };
  const { container } = render(<LocaleProvider locale="en"><HomeMediaPrompt brief="overview" /></LocaleProvider>);
  const photo = screen.getByRole("img", { name: homeMediaBriefsEn.overview.title });
  expect(photo).toHaveAttribute("src", "/assets/images/home/overview.webp");
  expect(photo).toHaveAttribute("loading", "lazy");
  expect(container.querySelectorAll("img")).toHaveLength(1);
  expect(container.querySelector("[data-kind]")).toBeNull();
  expect(screen.queryByRole("button")).not.toBeInTheDocument();
});
