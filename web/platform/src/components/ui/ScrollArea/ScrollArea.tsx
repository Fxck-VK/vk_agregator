"use client";

import {
  createElement,
  forwardRef,
  useCallback,
  useEffect,
  useRef,
  type ChangeEventHandler,
  type FormEvent,
  type HTMLAttributes,
  type PointerEvent,
  type ReactNode,
  type Ref,
  type TextareaHTMLAttributes,
  type UIEvent,
  type WheelEvent,
} from "react";

import styles from "./ScrollArea.module.css";

const MINIMUM_THUMB_SIZE = 32;
const SCROLL_IDLE_DELAY = 600;

type DataAttributes = {
  [key: `data-${string}`]: string | number | boolean | undefined;
};

type ScrollAreaTextareaProps = Pick<
  TextareaHTMLAttributes<HTMLTextAreaElement>,
  | "autoComplete"
  | "defaultValue"
  | "disabled"
  | "maxLength"
  | "minLength"
  | "name"
  | "placeholder"
  | "readOnly"
  | "required"
  | "rows"
  | "spellCheck"
  | "value"
  | "wrap"
> & {
  onChange?: ChangeEventHandler<HTMLTextAreaElement>;
};

type ScrollAreaViewportProps = Omit<
  HTMLAttributes<HTMLElement>,
  "children" | "className" | "onChange"
> & DataAttributes & ScrollAreaTextareaProps;

type ScrollAreaProps = Omit<HTMLAttributes<HTMLDivElement>, "children"> &
  DataAttributes & {
    children?: ReactNode;
    orientation?: "horizontal" | "vertical";
    trackPlacement?: "inset" | "outside";
    viewportAs?: "aside" | "div" | "main" | "nav" | "pre" | "section" | "textarea" | "ul";
    viewportClassName?: string;
    viewportProps?: ScrollAreaViewportProps;
    viewportRef?: Ref<HTMLElement>;
  };

type DragState = {
  pointerID: number;
  startPointerPosition: number;
  startScrollPosition: number;
};

type ScrollAreaOrientation = NonNullable<ScrollAreaProps["orientation"]>;

function getViewportSize(viewport: HTMLElement, orientation: ScrollAreaOrientation) {
  return orientation === "horizontal" ? viewport.clientWidth : viewport.clientHeight;
}

function getContentSize(viewport: HTMLElement, orientation: ScrollAreaOrientation) {
  return orientation === "horizontal" ? viewport.scrollWidth : viewport.scrollHeight;
}

function getTrackSize(track: HTMLElement, orientation: ScrollAreaOrientation) {
  return orientation === "horizontal" ? track.clientWidth : track.clientHeight;
}

function getScrollPosition(viewport: HTMLElement, orientation: ScrollAreaOrientation) {
  return orientation === "horizontal" ? viewport.scrollLeft : viewport.scrollTop;
}

function setScrollPosition(
  viewport: HTMLElement,
  orientation: ScrollAreaOrientation,
  value: number,
) {
  if (orientation === "horizontal") {
    viewport.scrollLeft = value;
  } else {
    viewport.scrollTop = value;
  }
}

function getPointerPosition(event: PointerEvent<HTMLDivElement>, orientation: ScrollAreaOrientation) {
  return orientation === "horizontal" ? event.clientX : event.clientY;
}

function assignRef<T>(ref: Ref<T> | undefined, value: T | null) {
  if (typeof ref === "function") {
    ref(value);
  } else if (ref) {
    ref.current = value;
  }
}

function roundPixelValue(value: number) {
  return Math.round(value * 1000) / 1000;
}

export const ScrollArea = forwardRef<HTMLDivElement, ScrollAreaProps>(function ScrollArea(
  {
    children,
    className,
    orientation = "vertical",
    trackPlacement = "inset",
    viewportAs = "div",
    viewportClassName,
    viewportProps,
    viewportRef,
    ...rootProps
  },
  forwardedRef,
) {
  const rootRef = useRef<HTMLDivElement>(null);
  const internalViewportRef = useRef<HTMLElement>(null);
  const trackRef = useRef<HTMLDivElement>(null);
  const thumbRef = useRef<HTMLDivElement>(null);
  const animationFrameRef = useRef<number | null>(null);
  const idleTimerRef = useRef<number | null>(null);
  const dragStateRef = useRef<DragState | null>(null);
  const {
    onInput: viewportOnInput,
    onScroll: viewportOnScroll,
    onWheel: viewportOnWheel,
    ...restViewportProps
  } = viewportProps ?? {};

  const setRootRef = useCallback((node: HTMLDivElement | null) => {
    rootRef.current = node;
    assignRef(forwardedRef, node);
  }, [forwardedRef]);

  const setViewportRef = useCallback((node: HTMLElement | null) => {
    internalViewportRef.current = node;
    assignRef(viewportRef, node);
  }, [viewportRef]);

  const updateThumb = useCallback(() => {
    const root = rootRef.current;
    const viewport = internalViewportRef.current;
    const track = trackRef.current;
    const thumb = thumbRef.current;
    if (!root || !viewport || !track || !thumb) return;

    const viewportSize = getViewportSize(viewport, orientation);
    const contentSize = getContentSize(viewport, orientation);
    const trackSize = getTrackSize(track, orientation) || viewportSize;
    const isScrollable = viewportSize > 0 && contentSize > viewportSize && trackSize > 0;
    root.dataset.scrollable = String(isScrollable);

    if (!isScrollable) {
      thumb.style.setProperty("--scroll-area-thumb-size", "0px");
      thumb.style.setProperty("--scroll-area-thumb-offset", "0px");
      return;
    }

    const thumbSize = Math.min(
      trackSize,
      Math.max(MINIMUM_THUMB_SIZE, trackSize * (viewportSize / contentSize)),
    );
    const scrollRange = contentSize - viewportSize;
    const thumbRange = trackSize - thumbSize;
    const boundedScrollPosition = Math.min(
      Math.max(getScrollPosition(viewport, orientation), 0),
      scrollRange,
    );
    const thumbOffset = scrollRange > 0
      ? (boundedScrollPosition / scrollRange) * thumbRange
      : 0;

    thumb.style.setProperty("--scroll-area-thumb-size", `${roundPixelValue(thumbSize)}px`);
    thumb.style.setProperty("--scroll-area-thumb-offset", `${roundPixelValue(thumbOffset)}px`);
  }, [orientation]);

  const scheduleThumbUpdate = useCallback(() => {
    if (animationFrameRef.current !== null) return;
    animationFrameRef.current = window.requestAnimationFrame(() => {
      animationFrameRef.current = null;
      updateThumb();
    });
  }, [updateThumb]);

  const markScrolling = useCallback(() => {
    const root = rootRef.current;
    if (!root) return;

    root.dataset.scrolling = "true";
    if (idleTimerRef.current !== null) window.clearTimeout(idleTimerRef.current);
    idleTimerRef.current = window.setTimeout(() => {
      delete root.dataset.scrolling;
      idleTimerRef.current = null;
    }, SCROLL_IDLE_DELAY);
  }, []);

  const handleNativeWheel = useCallback((event: globalThis.WheelEvent) => {
    // The floating track is a sibling of the viewport, so native scrolling
    // cannot reach the viewport when the pointer is over the track or thumb.
    const isTrackWheel = event.currentTarget === trackRef.current;
    if (
      event.defaultPrevented
      || event.ctrlKey
      || (!isTrackWheel && (
        orientation !== "horizontal"
        || Math.abs(event.deltaX) >= Math.abs(event.deltaY)
      ))
    ) {
      return;
    }

    const delta = orientation === "horizontal" && Math.abs(event.deltaX) > Math.abs(event.deltaY)
      ? event.deltaX
      : event.deltaY;
    if (delta === 0) return;

    const viewport = internalViewportRef.current;
    if (!viewport) return;

    const viewportSize = getViewportSize(viewport, orientation);
    const scrollRange = getContentSize(viewport, orientation) - viewportSize;
    if (scrollRange <= 0) return;

    const currentScrollPosition = Math.min(
      Math.max(getScrollPosition(viewport, orientation), 0),
      scrollRange,
    );
    const wheelMultiplier = event.deltaMode === globalThis.WheelEvent.DOM_DELTA_LINE
      ? 16
      : event.deltaMode === globalThis.WheelEvent.DOM_DELTA_PAGE
        ? viewportSize
        : 1;
    const nextScrollPosition = Math.min(
      Math.max(currentScrollPosition + delta * wheelMultiplier, 0),
      scrollRange,
    );
    if (nextScrollPosition === currentScrollPosition) return;

    event.preventDefault();
    setScrollPosition(viewport, orientation, nextScrollPosition);
    markScrolling();
    updateThumb();
    scheduleThumbUpdate();
  }, [markScrolling, orientation, scheduleThumbUpdate, updateThumb]);

  useEffect(() => {
    const viewport = internalViewportRef.current;
    const track = trackRef.current;
    if (!viewport) return;

    updateThumb();
    scheduleThumbUpdate();
    const resizeObserver = typeof ResizeObserver === "undefined"
      ? null
      : new ResizeObserver(scheduleThumbUpdate);
    resizeObserver?.observe(viewport);

    const mutationObserver = new MutationObserver(scheduleThumbUpdate);
    mutationObserver.observe(viewport, { childList: true, subtree: true });
    viewport.addEventListener("load", scheduleThumbUpdate, true);
    viewport.addEventListener("wheel", handleNativeWheel, { passive: false });
    track?.addEventListener("wheel", handleNativeWheel, { passive: false });
    window.addEventListener("resize", scheduleThumbUpdate);

    return () => {
      resizeObserver?.disconnect();
      mutationObserver.disconnect();
      viewport.removeEventListener("load", scheduleThumbUpdate, true);
      viewport.removeEventListener("wheel", handleNativeWheel);
      track?.removeEventListener("wheel", handleNativeWheel);
      window.removeEventListener("resize", scheduleThumbUpdate);
      if (animationFrameRef.current !== null) {
        window.cancelAnimationFrame(animationFrameRef.current);
      }
      if (idleTimerRef.current !== null) window.clearTimeout(idleTimerRef.current);
    };
  }, [handleNativeWheel, scheduleThumbUpdate, updateThumb]);

  const handleScroll = (event: UIEvent<HTMLElement>) => {
    viewportOnScroll?.(event);
    markScrolling();
    updateThumb();
    scheduleThumbUpdate();
  };

  const handleInput = (event: FormEvent<HTMLElement>) => {
    viewportOnInput?.(event);
    updateThumb();
    scheduleThumbUpdate();
  };

  const handleWheel = (event: WheelEvent<HTMLElement>) => {
    viewportOnWheel?.(event);
  };

  const handleTrackPointerDown = (event: PointerEvent<HTMLDivElement>) => {
    if (event.target !== event.currentTarget) return;

    const viewport = internalViewportRef.current;
    const track = trackRef.current;
    if (!viewport || !track) return;

    const viewportSize = getViewportSize(viewport, orientation);
    const contentSize = getContentSize(viewport, orientation);
    if (contentSize <= viewportSize) return;

    event.preventDefault();
    const trackRect = track.getBoundingClientRect();
    const trackSize = getTrackSize(track, orientation)
      || (orientation === "horizontal" ? trackRect.width : trackRect.height);
    const thumbSize = Math.min(
      trackSize,
      Math.max(MINIMUM_THUMB_SIZE, trackSize * (viewportSize / contentSize)),
    );
    const thumbRange = trackSize - thumbSize;
    const scrollRange = contentSize - viewportSize;
    const trackStart = orientation === "horizontal" ? trackRect.left : trackRect.top;
    const requestedThumbOffset = Math.min(
      Math.max(getPointerPosition(event, orientation) - trackStart - thumbSize / 2, 0),
      thumbRange,
    );

    setScrollPosition(
      viewport,
      orientation,
      thumbRange > 0 ? (requestedThumbOffset / thumbRange) * scrollRange : 0,
    );
    markScrolling();
    scheduleThumbUpdate();
  };

  const handleThumbPointerDown = (event: PointerEvent<HTMLDivElement>) => {
    const viewport = internalViewportRef.current;
    const root = rootRef.current;
    if (!viewport || !root) return;

    event.preventDefault();
    event.stopPropagation();
    event.currentTarget.setPointerCapture?.(event.pointerId);
    dragStateRef.current = {
      pointerID: event.pointerId,
      startPointerPosition: getPointerPosition(event, orientation),
      startScrollPosition: getScrollPosition(viewport, orientation),
    };
    root.dataset.dragging = "true";
  };

  const handleThumbPointerMove = (event: PointerEvent<HTMLDivElement>) => {
    const dragState = dragStateRef.current;
    const viewport = internalViewportRef.current;
    const track = trackRef.current;
    if (!dragState || dragState.pointerID !== event.pointerId || !viewport || !track) return;

    const viewportSize = getViewportSize(viewport, orientation);
    const contentSize = getContentSize(viewport, orientation);
    const trackSize = getTrackSize(track, orientation);
    const thumbSize = Math.min(
      trackSize,
      Math.max(MINIMUM_THUMB_SIZE, trackSize * (viewportSize / contentSize)),
    );
    const thumbRange = trackSize - thumbSize;
    const scrollRange = contentSize - viewportSize;
    if (thumbRange <= 0 || scrollRange <= 0) return;

    setScrollPosition(
      viewport,
      orientation,
      dragState.startScrollPosition
        + (getPointerPosition(event, orientation) - dragState.startPointerPosition)
          * (scrollRange / thumbRange),
    );
    markScrolling();
    scheduleThumbUpdate();
  };

  const finishDragging = (event: PointerEvent<HTMLDivElement>) => {
    const dragState = dragStateRef.current;
    if (!dragState || dragState.pointerID !== event.pointerId) return;

    event.currentTarget.releasePointerCapture?.(event.pointerId);
    dragStateRef.current = null;
    if (rootRef.current) delete rootRef.current.dataset.dragging;
  };

  const viewportElementProps = {
    ...restViewportProps,
    className: [styles.viewport, viewportClassName].filter(Boolean).join(" "),
    "data-scroll-area-viewport": "true",
    onInput: handleInput,
    onScroll: handleScroll,
    onWheel: handleWheel,
    ref: setViewportRef,
  };
  const viewport = viewportAs === "textarea"
    ? createElement(viewportAs, viewportElementProps)
    : createElement(viewportAs, viewportElementProps, children);

  return (
    <div
      {...rootProps}
      className={[styles.root, className].filter(Boolean).join(" ")}
      data-orientation={orientation}
      data-track-placement={trackPlacement}
      ref={setRootRef}
    >
      <div className={styles.frame}>
        {viewport}
        <div
          aria-hidden="true"
          className={styles.track}
          data-testid="floating-scrollbar-track"
          onPointerDown={handleTrackPointerDown}
          ref={trackRef}
        >
          <div
            className={styles.thumb}
            data-testid="floating-scrollbar-thumb"
            onPointerCancel={finishDragging}
            onPointerDown={handleThumbPointerDown}
            onPointerMove={handleThumbPointerMove}
            onPointerUp={finishDragging}
            ref={thumbRef}
          />
        </div>
      </div>
    </div>
  );
});
