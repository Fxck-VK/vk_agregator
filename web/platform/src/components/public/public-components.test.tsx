import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { ContentCard } from "./ContentCard/ContentCard";
import { EmptyState } from "./EmptyState/EmptyState";
import { FAQ } from "./FAQ/FAQ";
import { ModelPreviewCard } from "./ModelPreviewCard/ModelPreviewCard";
import { PageContainer } from "./PageContainer/PageContainer";
import { PrimaryButton } from "./PrimaryButton/PrimaryButton";
import { SecondaryButton } from "./SecondaryButton/SecondaryButton";
import { SectionHeading } from "./SectionHeading/SectionHeading";

describe("public design primitives", () => {
  it("renders a sized page container without changing its content semantics", () => {
    render(<PageContainer size="wide"><p>Contained content</p></PageContainer>);

    expect(screen.getByText("Contained content").parentElement).toHaveAttribute("data-size", "wide");
  });

  it("renders a section heading with optional supporting content", () => {
    render(
      <SectionHeading
        action={<a href="/models">Все модели</a>}
        description="Короткое описание"
        eyebrow="Каталог"
        title="Нейросети"
      />,
    );

    expect(screen.getByRole("heading", { level: 2, name: "Нейросети" })).toBeInTheDocument();
    expect(screen.getByText("Каталог")).toBeInTheDocument();
    expect(screen.getByText("Короткое описание")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Все модели" })).toHaveAttribute("href", "/models");
  });

  it("renders primary and secondary actions as real links", () => {
    render(
      <>
        <PrimaryButton href="/app">Открыть платформу</PrimaryButton>
        <SecondaryButton href="/models">Посмотреть модели</SecondaryButton>
      </>,
    );

    expect(screen.getByRole("link", { name: "Открыть платформу" })).toHaveAttribute("href", "/app");
    expect(screen.getByRole("link", { name: "Посмотреть модели" })).toHaveAttribute("href", "/models");
  });

  it("renders neutral and model-specific cards with semantic article markup", () => {
    render(
      <>
        <ContentCard><h2>Нейтральная карточка</h2></ContentCard>
        <ModelPreviewCard
          actionLabel="Подробнее"
          description="Описание модели"
          href="/models/example"
          title="Тестовая модель"
        />
      </>,
    );

    expect(screen.getByRole("heading", { name: "Нейтральная карточка" }).closest("article")).not.toBeNull();
    expect(screen.getByRole("heading", { name: "Тестовая модель" }).closest("article")).not.toBeNull();
    expect(screen.getByRole("link", { name: "Подробнее" })).toHaveAttribute("href", "/models/example");
  });

  it("renders native FAQ disclosure controls", () => {
    render(<FAQ items={[{ answer: "Ответ", question: "Вопрос?" }]} />);

    expect(screen.getByText("Вопрос?").closest("summary")).not.toBeNull();
    expect(screen.getByText("Ответ")).toBeInTheDocument();
  });

  it("renders an accessible empty state and optional action", () => {
    render(
      <EmptyState
        action={<a href="/app">Начать работу</a>}
        description="Здесь пока нет материалов"
        title="Пусто"
      />,
    );

    expect(screen.getByRole("status")).toHaveTextContent("Пусто");
    expect(screen.getByRole("link", { name: "Начать работу" })).toHaveAttribute("href", "/app");
  });
});
