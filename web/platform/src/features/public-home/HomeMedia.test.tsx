import { render, screen } from "@testing-library/react";
import { expect, it } from "vitest";
import { LocaleProvider } from "@/i18n/LocaleProvider";
import { homeMediaBriefsEn } from "@/i18n/public/home-media-en";
import { HomeMediaPrompt } from "./HomeMedia";

it("renders an accessible visible image brief without fetching a reference asset", () => {
  const { container } = render(<LocaleProvider locale="en"><HomeMediaPrompt brief="suiteImage" /></LocaleProvider>);
  expect(screen.getByRole("figure")).toHaveAccessibleName(/Image prompt/);
  expect(screen.getByText(homeMediaBriefsEn.suiteImage.prompt)).toBeVisible();
  expect(screen.getByText(`Format ${homeMediaBriefsEn.suiteImage.format}`)).toBeVisible();
  expect(container.querySelector("img,video,source")).toBeNull();
});
