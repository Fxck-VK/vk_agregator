import { cleanup, render } from "@testing-library/react";
import { afterEach, expect, it } from "vitest";

import { brandName } from "@/config/brand";
import { FAQ } from "./FAQ";

afterEach(cleanup);

it.each([`Что такое ${brandName}?`, `What is ${brandName}?`])("keeps the whole question in one layout item: %s", question => {
  const { container } = render(<FAQ items={[{ question, answer: "Answer" }]} />);
  const summary = container.querySelector("summary")!;
  const brand = summary.querySelector("[data-brand-name]")!;
  const questionBlock = brand.parentElement!;

  // Brand decoration must not split the question into separate flex items.
  expect(questionBlock).not.toBe(summary);
  expect(questionBlock.textContent).toBe(question);
  expect(questionBlock.parentElement).toBe(summary);
  expect(summary.children).toHaveLength(2);
  expect(summary.lastElementChild?.tagName).toBe("IMG");
});
