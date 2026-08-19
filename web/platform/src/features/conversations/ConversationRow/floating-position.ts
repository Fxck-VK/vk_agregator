type FloatingAnchorRect = {
  bottom: number;
  left: number;
  right: number;
  top: number;
};

type FloatingPanelSize = {
  height: number;
  width: number;
};

type FloatingViewport = {
  height: number;
  width: number;
};

const viewportMargin = 12;
const triggerGap = 8;

function clamp(value: number, minimum: number, maximum: number): number {
  return Math.min(Math.max(value, minimum), Math.max(minimum, maximum));
}

export function resolveFloatingPosition(
  anchor: FloatingAnchorRect,
  panel: FloatingPanelSize,
  viewport: FloatingViewport,
): { left: number; top: number } {
  const rightPlacement = anchor.right + triggerGap;
  const leftPlacement = anchor.left - triggerGap - panel.width;
  const left = rightPlacement + panel.width <= viewport.width - viewportMargin
    ? rightPlacement
    : clamp(leftPlacement, viewportMargin, viewport.width - panel.width - viewportMargin);

  const top = anchor.top + panel.height <= viewport.height - viewportMargin
    ? anchor.top
    : clamp(anchor.bottom - panel.height, viewportMargin, viewport.height - panel.height - viewportMargin);

  return { left: Math.round(left), top: Math.round(top) };
}
