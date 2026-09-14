"use client";

import Image from "next/image";
import Link from "next/link";
import {
  useCallback,
  useEffect,
  useId,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  type CSSProperties,
} from "react";
import { createPortal } from "react-dom";

import { assetPaths } from "@/assets/asset-paths";
import { SearchIcon } from "@/components/icons/SearchIcon";
import { ScrollArea } from "@/components/ui/ScrollArea/ScrollArea";
import { ru } from "@/i18n/ru";

import { getModelPresentation } from "../ModelCard/model-card-content";
import { ModelCard, type ModelCardModel } from "../ModelCard/ModelCard";
import { ModelIcon } from "../ModelIcon/ModelIcon";
import styles from "./WorkspaceModelSelector.module.css";

export type ModelSelectorCategory = "popular" | "images" | "text" | "video" | "audio";
export type ModelSelectorModel = ModelCardModel & {
  category: Exclude<ModelSelectorCategory, "popular">;
};
export type ModelSelectorStatus = "loading" | "ready" | "failure";
export type ModelSelectorVariant = "compact" | "panel" | "composer";

type ModelSelectorProps = {
  className?: string;
  dialogId?: string;
  dialogLabel?: string;
  disabled?: boolean;
  hideEmptySections?: boolean;
  models: readonly ModelSelectorModel[];
  onSelect: (model: ModelSelectorModel) => void;
  renderInPortal?: boolean;
  selectedModelId: string;
  status?: ModelSelectorStatus;
  triggerAriaLabel?: (name: string, isOpen: boolean) => string;
  variant?: ModelSelectorVariant;
};

type PopoverState = "closed" | "open" | "closing";
type PortalLayout = {
  inlineSize: number;
  insetBlockEnd: CSSProperties["insetBlockEnd"];
  insetBlockStart: CSSProperties["insetBlockStart"];
  insetInlineStart: number;
  maxBlockSize: number;
  placement: "bottom" | "top";
};

const popularModelLimit = 2;
const compactNameLimit = 40;
const portalEdge = 16;
const popoverGap = 12;
const popoverMaximumWidth = 512;
const popoverMaximumHeight = 736;
const portalLayer = 170;

const categoryOrder: readonly ModelSelectorCategory[] = [
  "popular",
  "images",
  "text",
  "video",
  "audio",
];

const categoryLabels: Readonly<Record<ModelSelectorCategory, string>> = {
  popular: ru.modelSelector.categories.popular,
  images: ru.modelSelector.categories.images,
  text: ru.modelSelector.categories.text,
  video: ru.modelSelector.categories.video,
  audio: ru.modelSelector.categories.audio,
};

function getDefaultTriggerAriaLabel(name: string, isOpen: boolean) {
  return `Выбрана нейросеть ${name}. ${isOpen ? "Закрыть" : "Открыть"} список`;
}

export function ModelSelector({
  className,
  dialogId: providedDialogId,
  dialogLabel = ru.modelSelector.dialogLabel,
  disabled = false,
  hideEmptySections = false,
  models,
  onSelect,
  renderInPortal = false,
  selectedModelId,
  status = "ready",
  triggerAriaLabel = getDefaultTriggerAriaLabel,
  variant = "compact",
}: Readonly<ModelSelectorProps>) {
  const generatedId = useId().replace(/[^a-zA-Z0-9_-]/g, "");
  const dialogId = providedDialogId ?? `model-selector-dialog-${generatedId}`;
  const rootRef = useRef<HTMLDivElement>(null);
  const popoverRef = useRef<HTMLElement>(null);
  const searchRef = useRef<HTMLInputElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const hasFocusedOnOpen = useRef(false);
  const [query, setQuery] = useState("");
  const [popoverState, setPopoverState] = useState<PopoverState>("closed");
  const [portalLayout, setPortalLayout] = useState<PortalLayout | null>(null);
  const isOpen = popoverState === "open";
  const selectedModel = models.find((model) => model.id === selectedModelId) ?? null;
  const selectedModelPresentation = selectedModel === null
    ? null
    : getModelPresentation(selectedModel);
  const triggerText = status === "loading"
    ? ru.modelSelector.loadingShort
    : (selectedModel?.name ?? ru.modelSelector.unavailable);
  const triggerCharacters = Array.from(triggerText);
  const visibleTriggerText = variant === "compact" && triggerCharacters.length > compactNameLimit
    ? `${triggerCharacters.slice(0, compactNameLimit - 1).join("")}…`
    : triggerText;

  const requestClose = useCallback((restoreTriggerFocus = false) => {
    setPopoverState((currentState) => (
      currentState === "closed" ? currentState : "closing"
    ));
    setQuery("");
    if (restoreTriggerFocus) {
      triggerRef.current?.focus();
    }
  }, []);

  const finishClosing = useCallback(() => {
    setPopoverState((currentState) => (
      currentState === "closing" ? "closed" : currentState
    ));
  }, []);

  useEffect(() => {
    if (!isOpen) {
      hasFocusedOnOpen.current = false;
      return;
    }
    if (hasFocusedOnOpen.current || (renderInPortal && portalLayout === null)) return;
    searchRef.current?.focus({ preventScroll: true });
    hasFocusedOnOpen.current = true;
  }, [isOpen, portalLayout, renderInPortal]);

  useEffect(() => {
    const popover = popoverRef.current;
    if (!popover) return;

    const closeAfterAnimation = (event: globalThis.AnimationEvent) => {
      const isPopoverExit = !event.animationName
        || event.animationName.includes("workspaceModelSelectorClose");
      if (
        popover.dataset.state === "closing"
        && event.target === popover
        && isPopoverExit
      ) {
        finishClosing();
      }
    };

    popover.addEventListener("animationend", closeAfterAnimation);
    return () => popover.removeEventListener("animationend", closeAfterAnimation);
  }, [finishClosing, popoverState]);

  useEffect(() => {
    if (!isOpen) return;

    const closeFromOutside = (event: PointerEvent) => {
      if (!(event.target instanceof Node)) return;
      if (
        !rootRef.current?.contains(event.target)
        && !popoverRef.current?.contains(event.target)
      ) {
        requestClose();
      }
    };
    const closeFromKeyboard = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        requestClose(true);
      }
    };

    document.addEventListener("pointerdown", closeFromOutside);
    document.addEventListener("keydown", closeFromKeyboard);
    return () => {
      document.removeEventListener("pointerdown", closeFromOutside);
      document.removeEventListener("keydown", closeFromKeyboard);
    };
  }, [isOpen, requestClose]);

  useLayoutEffect(() => {
    if (!renderInPortal || popoverState === "closed") {
      return;
    }

    const updateLayout = () => {
      const trigger = triggerRef.current;
      const popover = popoverRef.current;
      if (!trigger || !popover) return;

      const triggerBounds = trigger.getBoundingClientRect();
      const viewportWidth = document.documentElement.clientWidth || window.innerWidth;
      const viewportHeight = document.documentElement.clientHeight || window.innerHeight;
      const inlineSize = Math.min(popoverMaximumWidth, viewportWidth - portalEdge * 2);
      const insetInlineStart = Math.min(
        Math.max(portalEdge, triggerBounds.left),
        Math.max(portalEdge, viewportWidth - inlineSize - portalEdge),
      );
      const spaceBelow = Math.max(0, viewportHeight - triggerBounds.bottom - popoverGap - portalEdge);
      const spaceAbove = Math.max(0, triggerBounds.top - popoverGap - portalEdge);
      const desiredHeight = Math.min(
        popover.scrollHeight || popoverMaximumHeight,
        popoverMaximumHeight,
        viewportHeight - portalEdge * 2,
      );
      const placement = spaceBelow >= desiredHeight || spaceBelow >= spaceAbove
        ? "bottom"
        : "top";
      const availableHeight = placement === "bottom" ? spaceBelow : spaceAbove;

      setPortalLayout({
        inlineSize,
        insetBlockEnd: placement === "top"
          ? viewportHeight - triggerBounds.top + popoverGap
          : "auto",
        insetBlockStart: placement === "bottom" ? triggerBounds.bottom + popoverGap : "auto",
        insetInlineStart,
        maxBlockSize: Math.min(popoverMaximumHeight, Math.max(0, availableHeight)),
        placement,
      });
    };

    updateLayout();
    window.addEventListener("resize", updateLayout);
    window.addEventListener("scroll", updateLayout, true);
    return () => {
      window.removeEventListener("resize", updateLayout);
      window.removeEventListener("scroll", updateLayout, true);
    };
  }, [popoverState, renderInPortal]);

  const filteredModels = useMemo(() => {
    const normalizedQuery = query.trim().toLocaleLowerCase("ru");
    if (normalizedQuery.length === 0) return models;

    return models.filter((model) =>
      `${model.name} ${model.id}`.toLocaleLowerCase("ru").includes(normalizedQuery),
    );
  }, [models, query]);

  const modelSections = useMemo(() => {
    const popularModelIds = new Set(
      models.slice(0, popularModelLimit).map((model) => model.id),
    );

    return categoryOrder.map((category) => ({
      id: category,
      label: categoryLabels[category],
      models: category === "popular"
        ? filteredModels.filter((model) => popularModelIds.has(model.id))
        : filteredModels.filter((model) => (
          !popularModelIds.has(model.id) && model.category === category
        )),
    })).filter((section) => !hideEmptySections || section.models.length > 0);
  }, [filteredModels, hideEmptySections, models]);

  const triggerName = status === "loading"
    ? ru.modelSelector.loading
    : selectedModel === null
      ? ru.modelSelector.unavailable
      : triggerAriaLabel(selectedModel.name, isOpen);

  const selectModel = (model: ModelCardModel) => {
    const selected = models.find((candidate) => candidate.id === model.id);
    if (!selected) return;
    onSelect(selected);
    requestClose(true);
  };

  const popover = popoverState === "closed" ? null : (
    <section
      aria-hidden={popoverState === "closing" ? true : undefined}
      aria-label={dialogLabel}
      className={styles.popover}
      data-placement={portalLayout?.placement}
      data-state={popoverState}
      id={dialogId}
      inert={popoverState === "closing" ? true : undefined}
      ref={popoverRef}
      role="dialog"
      style={renderInPortal ? {
        inlineSize: portalLayout?.inlineSize,
        insetBlockEnd: portalLayout?.insetBlockEnd,
        insetBlockStart: portalLayout?.insetBlockStart,
        insetInlineStart: portalLayout?.insetInlineStart,
        maxBlockSize: portalLayout?.maxBlockSize,
        position: "fixed",
        visibility: portalLayout === null ? "hidden" : undefined,
        zIndex: portalLayer,
      } : undefined}
    >
      <div className={styles.searchRow}>
        <SearchIcon className={styles.searchIcon} />
        <input
          aria-label={ru.modelSelector.searchLabel}
          className={styles.search}
          onChange={(event) => setQuery(event.target.value)}
          placeholder={ru.modelSelector.searchPlaceholder}
          ref={searchRef}
          type="search"
          value={query}
        />
      </div>

      <ScrollArea className={styles.scrollArea} viewportClassName={styles.scrollViewport}>
        {filteredModels.length > 0 ? (
          <div className={styles.modelSections}>
            {modelSections.map((section) => {
              const headingId = `${dialogId}-${section.id}`;

              return (
                <section
                  aria-labelledby={headingId}
                  className={styles.modelSection}
                  key={section.id}
                >
                  <h2 className={styles.category} id={headingId}>{section.label}</h2>
                  {section.models.length > 0 ? (
                    <ul aria-label={section.label} className={styles.options}>
                      {section.models.map((model) => (
                        <li key={model.id}>
                          <ModelCard
                            model={model}
                            onActivate={selectModel}
                            selected={model.id === selectedModelId}
                            variant="selector"
                          />
                        </li>
                      ))}
                    </ul>
                  ) : (
                    <p className={styles.sectionEmpty}>{ru.modelSelector.sectionEmpty}</p>
                  )}
                </section>
              );
            })}
          </div>
        ) : (
          <p className={styles.empty} role="status">{ru.modelSelector.empty}</p>
        )}
      </ScrollArea>

      <Link
        className={styles.catalogueLink}
        href="/app/models"
        onClick={() => requestClose()}
        prefetch={false}
      >
        <span>{ru.modelSelector.openCatalogue}</span>
        <span aria-hidden="true">→</span>
      </Link>
    </section>
  );

  return (
    <div
      className={[styles.root, className].filter(Boolean).join(" ")}
      data-variant={variant}
      ref={rootRef}
    >
      <button
        aria-controls={dialogId}
        aria-expanded={isOpen}
        aria-haspopup="dialog"
        aria-label={triggerName}
        className={styles.trigger}
        disabled={disabled || status !== "ready" || selectedModel === null}
        onClick={() => {
          if (isOpen) {
            requestClose();
            return;
          }
          setPopoverState("open");
        }}
        ref={triggerRef}
        type="button"
      >
        <ModelIcon
          className={styles.modelIcon}
          src={selectedModelPresentation?.artworkSrc}
        />
        <span className={styles.triggerText}>
          {visibleTriggerText}
        </span>
        <span aria-hidden="true" className={`${styles.chevron} ${isOpen ? styles.chevronOpen : ""}`}>
          <Image alt="" height={8} src={assetPaths.icons.ui.faqArrow} unoptimized width={14} />
        </span>
      </button>

      {renderInPortal && popover !== null && typeof document !== "undefined"
        ? createPortal(popover, document.body)
        : popover}
    </div>
  );
}
