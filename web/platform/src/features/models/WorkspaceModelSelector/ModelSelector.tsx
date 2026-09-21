"use client";

import { StateNotice, LoadingIndicator } from "@/components/ui/AsyncState/AsyncState";
import { useMessages, useDictionary } from "@/i18n/LocaleProvider";


import { getTranslator, type Translator } from "@/i18n/messages";

import Image from "next/image";
import Link from "@/i18n/Link";
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
import { ModeSwitchPanel } from "@/components/ui/ModeSwitchPanel/ModeSwitchPanel";
import { ScrollArea } from "@/components/ui/ScrollArea/ScrollArea";
import selectableStyles from "@/components/ui/selectable-control.module.css";
import type { ChatModel } from "@/lib/web-api/contracts";

import { getModelPresentation } from "../ModelCard/model-card-content";
import type { ModelCardModel } from "../ModelCard/ModelCard";
import { ModelIcon } from "../ModelIcon/ModelIcon";
import { ModelSelectorOption } from "./ModelSelectorOption";
import {
  getModelSelectorSections,
  modelSelectorCategoryModelIds,
  type ModelSelectorCategoryId,
  type ModelSelectorCategoryModelIds,
} from "./model-selector-sections";
import styles from "./WorkspaceModelSelector.module.css";

export type ModelSelectorCategory = "popular" | "images" | "text" | "video" | "audio";
export type ModelSelectorModel = ModelCardModel & {
  capabilities?: ChatModel["capabilities"];
  categories?: string[];
  category: Exclude<ModelSelectorCategory, "popular">;
  isFree?: boolean;
  responsePrice?: { credits: number; maxOutputTokens?: number };
};
export type ModelSelectorStatus = "loading" | "ready" | "failure";
export type ModelSelectorVariant = "compact" | "panel" | "composer";

type ModelSelectorProps = {
  categoryErrors?: Partial<Record<ModelSelectorCategoryId, string>>;
  categoryModelIds?: ModelSelectorCategoryModelIds;
  className?: string;
  descriptionMode?: "inline" | "tooltip";
  dialogId?: string;
  dialogLabel?: string;
  disabled?: boolean;
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

const compactNameLimit = 40;
const portalEdge = 16;
const popoverGap = 12;
const popoverMaximumWidth = 512;
const popoverMaximumHeight = 736;
const composerPopoverHeight = 586;
const portalLayer = 170;

function getDefaultTriggerAriaLabel(name: string, isOpen: boolean, msg: Translator = getTranslator("ru")) {
  return msg("modelSelector.selectedAiModelValueValueList", { value1: name, value2: isOpen ? msg("modelSelector.close") : msg("modelSelector.open") });
}

export function ModelSelector({
  categoryErrors,
  categoryModelIds = modelSelectorCategoryModelIds,
  className,
  descriptionMode = "inline",
  dialogId: providedDialogId,
  dialogLabel: requestedDialogLabel,
  disabled = false,
  models,
  onSelect,
  renderInPortal = false,
  selectedModelId,
  status = "ready",
  triggerAriaLabel: requestedTriggerAriaLabel,
  variant = "compact",
}: Readonly<ModelSelectorProps>) {
  const t = useDictionary();
  const msg = useMessages();
  const triggerAriaLabel = requestedTriggerAriaLabel ?? ((name: string, open: boolean) => getDefaultTriggerAriaLabel(name, open, msg));
  const dialogLabel = requestedDialogLabel ?? t.modelSelector.dialogLabel;
  const generatedId = useId().replace(/[^a-zA-Z0-9_-]/g, "");
  const dialogId = providedDialogId ?? `model-selector-dialog-${generatedId}`;
  const rootRef = useRef<HTMLDivElement>(null);
  const popoverRef = useRef<HTMLElement>(null);
  const searchRef = useRef<HTMLInputElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const hasFocusedOnOpen = useRef(false);
  const [query, setQuery] = useState("");
  const [category, setCategory] = useState<ModelSelectorCategoryId>("popular");
  const [activeDescriptionId, setActiveDescriptionId] = useState<string | null>(null);
  const listViewportRef = useRef<HTMLElement>(null);
  const [popoverState, setPopoverState] = useState<PopoverState>("closed");
  const [portalLayout, setPortalLayout] = useState<PortalLayout | null>(null);
  const isOpen = popoverState === "open";
  const selectedModel = models.find((model) => model.id === selectedModelId) ?? null;
  const selectedModelPresentation = selectedModel === null
    ? null
    : getModelPresentation(selectedModel, msg);
  const triggerText = status === "loading"
    ? t.modelSelector.loadingShort
    : (selectedModel?.name ?? t.modelSelector.unavailable);
  const triggerCharacters = Array.from(triggerText);
  const visibleTriggerText = variant === "compact" && triggerCharacters.length > compactNameLimit
    ? `${triggerCharacters.slice(0, compactNameLimit - 1).join("")}…`
    : triggerText;

  const sections = useMemo(() => getModelSelectorSections(models, categoryModelIds, categoryErrors, t), [models, categoryModelIds, categoryErrors, t]);
  const activeCategory = sections.some((section) => section.id === category) ? category : sections[0]?.id ?? "popular";
  const panelId = `${dialogId}-models`;
  const visibleSections = useMemo(() => {
    const normalizedQuery = query.trim().toLowerCase();
    const orderedSections = [
      ...sections.filter((section) => section.id === activeCategory),
      ...sections.filter((section) => section.id !== activeCategory),
    ];
    return orderedSections.map((section) => ({
      ...section,
      models: section.models.filter((model) =>
        `${model.name} ${model.id}`.toLowerCase().includes(normalizedQuery),
      ),
    })).filter((section) => section.models.length > 0 || section.error);
  }, [activeCategory, query, sections]);

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
        variant === "composer" ? composerPopoverHeight : popover.scrollHeight || popoverMaximumHeight,
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
        maxBlockSize: Math.min(
          variant === "composer" ? desiredHeight : popoverMaximumHeight,
          Math.max(0, availableHeight),
        ),
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
  }, [popoverState, renderInPortal, variant]);

  useLayoutEffect(() => {
    if (listViewportRef.current) listViewportRef.current.scrollTop = 0;
  }, [activeCategory, query, sections]);

  const promoteCategory = (id: ModelSelectorCategoryId) => {
    setCategory(id);
    setActiveDescriptionId(null);
    if (listViewportRef.current) listViewportRef.current.scrollTop = 0;
  };

  const triggerName = status === "loading"
    ? t.modelSelector.loading
    : selectedModel === null
      ? t.modelSelector.unavailable
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
        blockSize: variant === "composer" ? portalLayout?.maxBlockSize : undefined,
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
          aria-label={t.modelSelector.searchLabel}
          className={styles.search}
          onChange={(event) => {
            setQuery(event.target.value);
            setActiveDescriptionId(null);
          }}
          placeholder={t.modelSelector.searchPlaceholder}
          ref={searchRef}
          type="search"
          value={query}
        />
      </div>

      {sections.length > 0 ? <ModeSwitchPanel
        activeID={activeCategory}
        ariaLabel={t.modelsCatalog.categoryTabsLabel}
        className={styles.categoryPanel}
        items={sections.map(({ id, label }) => ({ id, label }))}
        onChange={promoteCategory}
      /> : null}

      <ScrollArea
        className={styles.scrollArea}
        viewportClassName={styles.scrollViewport}
        viewportProps={{ id: panelId, role: "region", "aria-label": t.modelSelector.feedLabel, tabIndex: 0 }}
        viewportRef={listViewportRef}
      >
        {visibleSections.length > 0 ? (
          <div className={styles.categoryFeed}>
            {visibleSections.map((section) => (
              <section aria-labelledby={`${panelId}-${section.id}`} key={section.id}>
                <h3 className={styles.categoryHeading} id={`${panelId}-${section.id}`}>{section.label}</h3>
                {section.error ? <StateNotice inline kind="empty">{section.error}</StateNotice> : null}
                {section.models.length > 0 ? <ul className={styles.options}>
                  {section.models.map((model) => (
                    <ModelSelectorOption
                      descriptionMode={descriptionMode}
                      isDescriptionActive={activeDescriptionId === `${section.id}:${model.id}`}
                      isOpen={isOpen}
                      key={model.id}
                      model={model}
                      onActivate={selectModel}
                      onDescriptionActivate={() => setActiveDescriptionId(`${section.id}:${model.id}`)}
                      popoverRef={popoverRef}
                      selected={model.id === selectedModelId}
                    />
                  ))}
                </ul> : null}
              </section>
            ))}
          </div>
        ) : (
          <StateNotice inline kind="empty">
            {t.modelSelector.empty}
          </StateNotice>
        )}
      </ScrollArea>

      <div className={`${selectableStyles.actionItems} ${styles.catalogueFooter}`}>
        <Link
          className={`${selectableStyles.control} ${styles.catalogueLink}`}
          href="/app/models"
          onClick={() => requestClose()}
          prefetch={false}
        >
          <span>{t.modelSelector.openCatalogue}</span>
        </Link>
      </div>
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
        {status === "loading" ? <LoadingIndicator label={t.modelSelector.loading} className={styles.modelIcon} /> : <ModelIcon className={styles.modelIcon} src={selectedModelPresentation?.artworkSrc} />}
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
