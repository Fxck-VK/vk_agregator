"use client";

import { cloneElement, useCallback, useEffect, useState, type CSSProperties, type ReactElement, type ReactNode } from "react";
import { LandingDialog } from "@/components/public/landing/LandingDialog";
import { LandingArrow } from "@/components/public/landing/LandingPrimitives";
import { Button } from "@/components/ui/Button/Button";
import { HomePicture } from "@/features/landing-variants/third/HomeMedia";
import palette from "@/features/landing-variants/third/Palette.module.css";
import { presentationExampleRu } from "@/i18n/presentation-example";
import { PresentationSelect } from "./PresentationSelect";
import { presentationColorStyles } from "./PresentationColors";
import { PresentationInspector } from "./PresentationInspector";
import { PresentationSettings, readPresentationAppearance, savePresentationAppearance, type PresentationAppearance } from "./PresentationSettings";
import { moduleStyles, presentationModule, slides, type ModuleAppearance, type Slide } from "./presentation-modules";
import styles from "./PresentationExample.module.css";

const copy = presentationExampleRu;

type Position = { slide: number; step: number; module?: string };

function move(position: Position, direction: -1 | 1, editing: boolean): Position {
  const { slide, step } = position;
  const nextStep = step + direction;
  if (!editing && nextStep >= 0 && nextStep <= slides[slide].steps) return { slide, step: nextStep };
  const nextSlide = slide + direction;
  if (nextSlide < 0 || nextSlide >= slides.length) return position;
  return { slide: nextSlide, step: direction > 0 || editing ? 0 : slides[nextSlide].steps };
}

export function PresentationExample() {
  const [open, setOpen] = useState(false);
  const close = useCallback(() => setOpen(false), []);

  return <div className={styles.preview}>
    <Button className={styles.launch} onClick={() => setOpen(true)}>{copy.launcher} <LandingArrow /></Button>
    {open && <LandingDialog title={copy.dialog.title} closeLabel={copy.dialog.closeLabel} className={`${palette.root} ${styles.dialog}`} wide onClose={close}>
      <ExamplePlayer />
    </LandingDialog>}
  </div>;
}

function ExamplePlayer() {
  const [position, setPosition] = useState<Position>({ slide: 0, step: 0 });
  const [appearance, setAppearance] = useState(readPresentationAppearance);
  const [editing, setEditing] = useState(false);
  const slide = slides[position.slide];
  const selected = editing ? position.module ?? "slide" : undefined;
  const moduleValues = appearance.modules[slide.value];
  const localBackground = moduleValues?.slide?.fill;
  const resolvedColors = localBackground ? { ...appearance.colors, background: localBackground } : appearance.colors;
  const lightByDefault = slide.value === "model" || slide.value === "summary";
  const light = appearance.background === "light" || appearance.background === "original" && lightByDefault;
  const navigate = useCallback((direction: -1 | 1) => setPosition(current => move(current, direction, editing)), [editing]);
  const selectModule = (module: string) => setPosition(current => ({ ...current, module }));

  function changeAppearance(value: PresentationAppearance) {
    setAppearance(value);
    savePresentationAppearance(value);
  }

  function changeModule(value?: ModuleAppearance) {
    if (!selected) return;
    const updated = { ...moduleValues };
    if (value && Object.values(value).some(option => option !== undefined)) updated[selected] = value;
    else delete updated[selected];
    changeAppearance({ ...appearance, modules: { ...appearance.modules, [slide.value]: updated } });
  }

  useEffect(() => {
    const handleKey = (event: KeyboardEvent) => {
      if (event.defaultPrevented || event.altKey || event.ctrlKey || event.metaKey || event.shiftKey) return;
      if (event.target instanceof Element && event.target.closest("input, textarea, select, [contenteditable=true]")) return;
      if (event.key === "ArrowRight" || event.key === "ArrowLeft") {
        event.preventDefault();
        navigate(event.key === "ArrowRight" ? 1 : -1);
      }
    };
    window.addEventListener("keydown", handleKey);
    return () => window.removeEventListener("keydown", handleKey);
  }, [navigate]);

  return <div className={styles.player}>
    <div className={styles.workspace} data-editing={editing || undefined}>
      <div className={styles.stage} data-slide-font={appearance.font} data-slide-background={appearance.background} data-slide-animation={editing ? "off" : appearance.animation} style={presentationColorStyles(resolvedColors)}>
        <div key={slide.value} className={styles.slide} data-slide={slide.value}>
          {editing && <ModuleButton slide={slide.value} id="slide" selected={selected} onSelect={selectModule} />}
          <SlideContents slide={slide.value} step={editing ? slide.steps : position.step} values={moduleValues} selected={selected} onSelect={selectModule} />
        </div>
      </div>
      {selected && <PresentationInspector key={`${slide.value}:${selected}`} slide={slide.value} selected={selected} saved={moduleValues?.[selected]} colors={resolvedColors} light={light} onChange={changeModule} />}
    </div>
    <div className={styles.controls}>
      <PresentationSelect label={copy.controls.slide} value={slide.value} options={slides} portalLayer={170} onChange={value => setPosition({ slide: slides.findIndex(item => item.value === value), step: 0 })} />
      <p className={styles.progress} role="status">{copy.controls.slide} {position.slide + 1} {copy.controls.of} {slides.length}{!editing && slide.steps > 0 && ` · ${copy.controls.step} ${position.step + 1} ${copy.controls.of} ${slide.steps + 1}`}</p>
      <div className={styles.nav}>
        <Button className={styles.editTrigger} variant="outline" aria-expanded={editing} onClick={() => setEditing(current => !current)}>{editing ? copy.controls.done : copy.controls.edit}</Button>
        <PresentationSettings value={appearance} table={slide.value === "model"} lightByDefault={lightByDefault} onChange={changeAppearance} />
        <Button variant="outline" aria-label={copy.controls.previous} disabled={position.slide === 0 && (editing || position.step === 0)} onClick={() => navigate(-1)}><LandingArrow direction="left" /></Button>
        <Button variant="outline" aria-label={copy.controls.next} disabled={position.slide === slides.length - 1 && (editing || position.step === slide.steps)} onClick={() => navigate(1)}><LandingArrow /></Button>
      </div>
    </div>
  </div>;
}

function ModuleButton({ slide, id, selected, onSelect }: { slide: Slide; id: string; selected?: string; onSelect: (id: string) => void }) {
  return <button type="button" className={styles.moduleButton} data-module={id} aria-label={`${copy.controls.selectPrefix}: ${presentationModule(slide, id).label}`} aria-pressed={id === selected} onClick={() => onSelect(id)} />;
}

function SlideContents({ slide, step, values, selected, onSelect }: { slide: Slide; step: number; values?: Record<string, ModuleAppearance>; selected?: string; onSelect: (id: string) => void }) {
  function module(id: string, content: ReactElement<{ children?: ReactNode; style?: CSSProperties }>) {
    const style = moduleStyles(values?.[id]);
    return selected
      ? cloneElement(content, { style, key: id }, content.props.children, <ModuleButton slide={slide} id={id} selected={selected} onSelect={onSelect} />)
      : cloneElement(content, { style, key: id });
  }

  switch (slide) {
    case "cover": return <>
      {module("image", <div className={styles.coverPhoto}><HomePicture photo="workflowStart" original /></div>)}
      <div className={styles.coverCopy}>
        {module("kicker", <p>{copy.example.cover.kicker}</p>)}
        {module("title", <h3>{copy.example.cover.title}</h3>)}
        {module("subtitle", <p>{copy.example.cover.subtitle}</p>)}
      </div>
    </>;
    case "model": return <>
      {module("title", <h3>{copy.example.model.title}</h3>)}
      {module("subtitle", <p className={styles.subtitle}>{copy.example.model.subtitle}</p>)}
      <table className={styles.table}>
        <thead><tr>{module("idHeading", <th scope="col" data-focus={step > 0}>{copy.example.model.idHeading}</th>)}{module("nameHeading", <th scope="col">{copy.example.model.nameHeading}</th>)}</tr></thead>
        <tbody>
          <tr>{module("firstId", <td data-focus={step > 0}>{copy.example.model.firstId}</td>)}{module("firstName", <td>{copy.example.model.firstName}</td>)}</tr>
          <tr>{module("secondId", <td data-focus={step > 0}>{copy.example.model.secondId}</td>)}{module("secondName", <td>{copy.example.model.secondName}</td>)}</tr>
        </tbody>
      </table>
      {module("note", <p data-reveal={step > 0} aria-hidden={step === 0}>{copy.example.model.note}</p>)}
    </>;
    case "relations": return <>
      {module("title", <h3>{copy.example.relations.title}</h3>)}
      <div className={styles.relations} data-linked={step >= 2}>
        {module("students", <div className={styles.entity}><strong>{copy.example.relations.students}</strong><span>{copy.example.relations.studentFields}</span></div>)}
        {module("grades", <div className={styles.entity} data-reveal={step >= 1} aria-hidden={step < 1}><strong>{copy.example.relations.grades}</strong><span>{copy.example.relations.gradeFields}</span><span>{copy.example.relations.grade}</span></div>)}
        {module("subjects", <div className={styles.entity}><strong>{copy.example.relations.subjects}</strong><span>{copy.example.relations.subjectFields}</span></div>)}
      </div>
      {module("note", <p data-reveal={step >= 2} aria-hidden={step < 2}>{copy.example.relations.note}</p>)}
    </>;
    case "security": return <>
      {module("title", <h3>{copy.example.security.title}</h3>)}
      <dl className={styles.protections}>{copy.example.security.protections.map(({ id, risk, measure }, index) => <div key={id} style={moduleStyles(values?.[id])}>
        <dt>{risk}</dt><dd data-reveal={step > index} aria-hidden={step <= index}>
          {measure}{selected && <ModuleButton slide={slide} id={id} selected={selected} onSelect={onSelect} />}
        </dd>
      </div>)}</dl>
    </>;
    case "summary": return <>
      {module("title", <h3>{copy.example.summary.title}</h3>)}
      <div className={styles.audience}>
        {module("student", <div><strong>{copy.example.summary.student}</strong><p>{copy.example.summary.studentDescription}</p></div>)}
        {module("teacher", <div data-reveal={step > 0} aria-hidden={step === 0}><strong>{copy.example.summary.teacher}</strong><p>{copy.example.summary.teacherDescription}</p></div>)}
      </div>
      {module("subtitle", <p className={styles.subtitle}>{copy.example.summary.subtitle}</p>)}
    </>;
  }
}
