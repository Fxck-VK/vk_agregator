import type { CSSProperties } from "react";
import { presentationExampleRu } from "@/i18n/presentation-example";
import { normalizeColor, readableInk, type ColorKey } from "./PresentationColors";

const copy = presentationExampleRu;
export const slides = copy.slides;

export type Slide = (typeof slides)[number]["value"];

export const fonts = [
  { id: "geist", label: "Geist", family: "var(--font-sans)" },
  { id: "arial", label: "Arial", family: "Arial, sans-serif" },
  { id: "georgia", label: "Georgia", family: "Georgia, serif" },
] as const;

export type PresentationFont = (typeof fonts)[number]["id"];

type Module = {
  label: string;
  kind: "background" | "text" | "block" | "image";
  tableColor?: Exclude<ColorKey, "background" | "text">;
};

const common = {
  slide: { label: copy.moduleLabels.common.slide, kind: "background" },
  title: { label: copy.moduleLabels.common.title, kind: "text" },
} satisfies Record<string, Module>;

const modulesBySlide: Record<Slide, Record<string, Module>> = {
  cover: {
    ...common,
    kicker: { label: copy.moduleLabels.cover.kicker, kind: "text" },
    subtitle: { label: copy.moduleLabels.cover.subtitle, kind: "text" },
    image: { label: copy.moduleLabels.cover.image, kind: "image" },
  },
  model: {
    ...common,
    subtitle: { label: copy.moduleLabels.model.subtitle, kind: "text" },
    note: { label: copy.moduleLabels.model.note, kind: "text" },
    idHeading: { label: copy.moduleLabels.model.idHeading, kind: "block", tableColor: "tableHighlight" },
    nameHeading: { label: copy.moduleLabels.model.nameHeading, kind: "block", tableColor: "tableHeader" },
    firstId: { label: copy.moduleLabels.model.firstId, kind: "block", tableColor: "tableHighlight" },
    firstName: { label: copy.moduleLabels.model.firstName, kind: "block", tableColor: "tableFill" },
    secondId: { label: copy.moduleLabels.model.secondId, kind: "block", tableColor: "tableHighlight" },
    secondName: { label: copy.moduleLabels.model.secondName, kind: "block", tableColor: "tableFill" },
  },
  relations: {
    ...common,
    students: { label: copy.moduleLabels.relations.students, kind: "block" },
    grades: { label: copy.moduleLabels.relations.grades, kind: "block" },
    subjects: { label: copy.moduleLabels.relations.subjects, kind: "block" },
    note: { label: copy.moduleLabels.relations.note, kind: "text" },
  },
  security: {
    ...common,
    permissions: { label: copy.moduleLabels.security.permissions, kind: "block" },
    injections: { label: copy.moduleLabels.security.injections, kind: "block" },
    loss: { label: copy.moduleLabels.security.loss, kind: "block" },
  },
  summary: {
    ...common,
    student: { label: copy.moduleLabels.summary.student, kind: "block" },
    teacher: { label: copy.moduleLabels.summary.teacher, kind: "block" },
    subtitle: { label: copy.moduleLabels.summary.subtitle, kind: "text" },
  },
};

export type ModuleAppearance = { fill?: string; ink?: string; font?: PresentationFont; opacity?: number };
export type PresentationModuleValues = Partial<Record<Slide, Record<string, ModuleAppearance>>>;

export function presentationModule(slide: Slide, id: string) {
  return modulesBySlide[slide][id];
}

export function parsePresentationModules(value: unknown): PresentationModuleValues {
  const result: PresentationModuleValues = {};
  if (!value || typeof value !== "object") return result;
  for (const { value: slide } of slides) {
    const saved = (value as Record<string, unknown>)[slide];
    if (!saved || typeof saved !== "object") continue;
    for (const [id, module] of Object.entries(modulesBySlide[slide])) {
      const raw = (saved as Record<string, unknown>)[id];
      if (!raw || typeof raw !== "object") continue;
      const data = raw as Record<string, unknown>;
      const appearance: ModuleAppearance = {};
      const fill = normalizeColor(data.fill);
      const ink = normalizeColor(data.ink);
      const font = fonts.find(option => option.id === data.font)?.id;
      if (fill && (module.kind === "background" || module.kind === "block")) appearance.fill = fill;
      if (ink && (module.kind === "text" || module.kind === "block")) appearance.ink = ink;
      if (font && (module.kind === "text" || module.kind === "block")) appearance.font = font;
      if (module.kind === "image" && typeof data.opacity === "number" && Number.isInteger(data.opacity) && data.opacity >= 0 && data.opacity <= 100) {
        appearance.opacity = data.opacity;
      }
      if (Object.keys(appearance).length) (result[slide] ??= {})[id] = appearance;
    }
  }
  return result;
}

export function moduleStyles(appearance?: ModuleAppearance): CSSProperties | undefined {
  if (!appearance) return;
  return {
    fontFamily: fonts.find(font => font.id === appearance.font)?.family,
    backgroundColor: appearance.fill,
    color: appearance.ink ?? readableInk(appearance.fill),
    "--illustration-opacity": appearance.opacity === undefined ? undefined : appearance.opacity / 100,
  } as CSSProperties;
}
