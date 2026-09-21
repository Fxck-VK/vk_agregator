"use client";

import { useMessages } from "@/i18n/LocaleProvider";
import { tokenAmount } from "@/i18n/counts";
import { RichMessage } from "@/i18n/RichMessage";
import { CreditAmount } from "@/components/ui/CreditAmount/CreditAmount";
import type { ModelSelectorModel } from "./ModelSelector";

import { useCallback, useId, useLayoutEffect, useRef, useState, type ReactNode, type RefObject } from "react";
import { createPortal } from "react-dom";

import { TooltipBubble } from "@/components/ui/Tooltip/Tooltip";

import { ModelCard, type ModelCardModel } from "../ModelCard/ModelCard";
import { getModelPresentation } from "../ModelCard/model-card-content";
import styles from "./WorkspaceModelSelector.module.css";

type Bounds = Pick<DOMRect, "left" | "right" | "top" | "bottom">;

export function getModelDescriptionPosition(
  anchor: Bounds,
  panel: Bounds,
  tooltip: { width: number; height: number },
  viewport: { width: number; height: number },
) {
  const edge = 16;
  const gap = 12;
  const clamp = (value: number, maximum: number) => Math.max(edge, Math.min(value, maximum));
  let left: number;
  let top = (anchor.top + anchor.bottom - tooltip.height) / 2;

  if (panel.right + gap + tooltip.width <= viewport.width - edge) {
    left = panel.right + gap;
  } else if (panel.left - gap - tooltip.width >= edge) {
    left = panel.left - gap - tooltip.width;
  } else {
    left = clamp(anchor.left, viewport.width - edge - tooltip.width);
    top = anchor.top - gap - tooltip.height >= edge
      ? anchor.top - gap - tooltip.height
      : anchor.bottom + gap;
  }

  return { left, top: clamp(top, viewport.height - edge - tooltip.height) };
}

function ModelDescriptionTooltip({ anchor, description, onDismiss, popoverRef }: {
  anchor: HTMLElement;
  description: ReactNode;
  onDismiss: () => void;
  popoverRef: RefObject<HTMLElement | null>;
}) {
  const tooltipRef = useRef<HTMLDivElement>(null);
  const [position, setPosition] = useState<{ left: number; top: number } | null>(null);

  useLayoutEffect(() => {
    const tooltip = tooltipRef.current;
    if (!tooltip) return;
    const anchorBounds = anchor.getBoundingClientRect();
    setPosition(getModelDescriptionPosition(
      anchorBounds,
      popoverRef.current?.getBoundingClientRect() ?? anchorBounds,
      tooltip.getBoundingClientRect(),
      {
        width: document.documentElement.clientWidth || window.innerWidth,
        height: document.documentElement.clientHeight || window.innerHeight,
      },
    ));
    window.addEventListener("scroll", onDismiss, true);
    window.addEventListener("resize", onDismiss);
    return () => {
      window.removeEventListener("scroll", onDismiss, true);
      window.removeEventListener("resize", onDismiss);
    };
  }, [anchor, description, onDismiss, popoverRef]);

  return createPortal(
    <div
      className={styles.descriptionTooltip}
      ref={tooltipRef}
      style={{ left: position?.left ?? 0, top: position?.top ?? 0, visibility: position ? undefined : "hidden" }}
    >
      <TooltipBubble className={styles.descriptionTooltipBubble}>{description}</TooltipBubble>
    </div>,
    document.body,
  );
}

export function ModelSelectorOption({ descriptionMode, isDescriptionActive, isOpen, model, onActivate, onDescriptionActivate, popoverRef, selected }: {
  descriptionMode: "inline" | "tooltip";
  isDescriptionActive: boolean;
  isOpen: boolean;
  model: ModelSelectorModel;
  onActivate: (model: ModelCardModel) => void;
  onDescriptionActivate: () => void;
  popoverRef: RefObject<HTMLElement | null>;
  selected: boolean;
}) {
  const msg = useMessages();
  const descriptionId = useId();
  const [activeDescription, setActiveDescription] = useState<{ source: "pointer" | "focus"; anchor: HTMLElement } | null>(null);
  const dismissDescription = useCallback(() => setActiveDescription(null), []);
  const showDescription = descriptionMode === "tooltip" && isOpen;
  const description = model.responsePrice ? <RichMessage id="chatModelSelector.valueTokensPerResponseValue" values={{
    value1: <CreditAmount value={model.responsePrice.credits} />,
    value2: model.responsePrice.maxOutputTokens ? msg("chatModelSelector.upToValueResponseTokens", { value1: tokenAmount(msg, model.responsePrice.maxOutputTokens, "outputTokens") }) : "",
  }} /> : getModelPresentation(model, msg).description;

  return (
    <li
      onBlur={() => setActiveDescription((active) => active?.source === "focus" ? null : active)}
      onFocus={(event) => {
        if (showDescription) {
          onDescriptionActivate();
          setActiveDescription({ source: "focus", anchor: event.currentTarget });
        }
      }}
      onPointerEnter={(event) => {
        if (showDescription && event.pointerType !== "touch") {
          onDescriptionActivate();
          setActiveDescription({ source: "pointer", anchor: event.currentTarget });
        }
      }}
      onPointerLeave={() => setActiveDescription((active) => active?.source === "pointer" ? null : active)}
    >
      <ModelCard
        descriptionContent={description}
        descriptionId={descriptionMode === "tooltip" ? descriptionId : undefined}
        descriptionMode={descriptionMode}
        model={model}
        onActivate={onActivate}
        selected={selected}
        variant="selector"
      />
      {showDescription && isDescriptionActive && activeDescription !== null ? (
        <ModelDescriptionTooltip
          anchor={activeDescription.anchor}
          description={description}
          onDismiss={dismissDescription}
          popoverRef={popoverRef}
        />
      ) : null}
    </li>
  );
}
