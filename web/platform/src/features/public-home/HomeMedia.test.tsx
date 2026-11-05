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
    sizes: "(max-width: 767px) 100vw, 800px" };
  const { container } = render(<LocaleProvider locale="en"><HomeMediaPrompt brief="hero" /></LocaleProvider>);
  const figure = screen.getByRole("figure", { name: homeMediaBriefsEn.hero.title });
  const photo = screen.getByRole("img", { name: homeMediaBriefsEn.hero.title });
  expect(photo).toHaveAttribute("src", "/assets/images/home/hero.webp");
  expect(photo).toHaveAttribute("width", "1600");
  expect(photo).toHaveAttribute("height", "900");
  expect(photo).toHaveAttribute("loading", "eager");
  expect(photo).toHaveAttribute("fetchpriority", "high");
  expect(photo).toHaveAttribute("srcset", assets.hero.srcSet);
  expect(photo).toHaveAttribute("sizes", assets.hero.sizes);
  expect(figure).toHaveStyle({ aspectRatio: "1600 / 900" });
  expect(screen.queryByText(homeMediaBriefsEn.hero.prompt)).not.toBeInTheDocument();
  expect(screen.queryByText(`Format ${homeMediaBriefsEn.hero.format}`)).not.toBeInTheDocument();
  expect(container.querySelector("video")).toBeNull();
});

it("keeps a real interface capture separate from the lazy suite artwork", () => {
  assets.suiteImage = {
    src: "/assets/images/home/suite-image.webp", width: 1600, height: 900,
    captures: [{ kind: "image", src: "/assets/images/home/image-workspace.png", width: 1440, height: 350 }],
  };
  const { container } = render(<LocaleProvider locale="en"><HomeMediaPrompt brief="suiteImage" /></LocaleProvider>);
  const figure = screen.getByRole("figure", { name: homeMediaBriefsEn.suiteImage.title });
  expect(figure).toHaveAttribute("data-brief", "suiteImage");
  const photo = screen.getByRole("img", { name: homeMediaBriefsEn.suiteImage.title });
  expect(photo).toHaveAttribute("loading", "lazy");
  const capture = container.querySelector('[data-kind="image"] img');
  expect(capture).toHaveAttribute("src", "/assets/images/home/image-workspace.png");
  expect(capture).toHaveAttribute("width", "1440");
  expect(capture).toHaveAttribute("height", "350");
  expect(capture).toHaveAttribute("loading", "lazy");
  expect(capture).toHaveAttribute("alt", "");
  expect(container.querySelectorAll("img")).toHaveLength(2);
  expect(screen.queryByText(homeMediaBriefsEn.suiteImage.prompt)).not.toBeInTheDocument();
});

it("renders both original overview captures without turning their contents into controls", () => {
  assets.overview = {
    src: "/assets/images/home/overview.webp", width: 1600, height: 900,
    captures: [
      { kind: "image", src: "/assets/images/home/image-workspace.png", width: 1440, height: 350 },
      { kind: "text", src: "/assets/images/home/text-workspace.png", width: 1440, height: 760 },
    ],
  };
  const { container } = render(<LocaleProvider locale="en"><HomeMediaPrompt brief="overview" /></LocaleProvider>);
  expect(container.querySelector('[data-kind="image"] img')).toHaveAttribute("src", "/assets/images/home/image-workspace.png");
  expect(container.querySelector('[data-kind="text"] img')).toHaveAttribute("src", "/assets/images/home/text-workspace.png");
  expect(container.querySelectorAll("img")).toHaveLength(3);
  expect(screen.queryByRole("button")).not.toBeInTheDocument();
});
