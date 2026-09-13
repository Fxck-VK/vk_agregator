import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type ChangeEventHandler,
  type KeyboardEvent,
} from "react";

import { ScrollArea } from "@/components/ui/ScrollArea/ScrollArea";

import styles from "./ChatTextInput.module.css";

type ChatTextInputAppearance = "composer" | "inset" | "plain";
type ChatTextInputSize = "compact" | "expanded";

const MAX_AUTO_ROWS = 9;
const MANUAL_TRANSITION_DURATION_MS = 360;

function pixelValue(value: string, fallback = 0) {
  const parsedValue = Number.parseFloat(value);
  return Number.isFinite(parsedValue) ? parsedValue : fallback;
}

type ChatTextInputProps = {
  appearance: ChatTextInputAppearance;
  disabled: boolean;
  onChange: ChangeEventHandler<HTMLTextAreaElement>;
  onSend: () => void;
  placeholder: string;
  rows: number;
  size: ChatTextInputSize;
  value: string;
};

export function ChatTextInput({
  appearance,
  disabled,
  onChange,
  onSend,
  placeholder,
  rows,
  size,
  value,
}: ChatTextInputProps) {
  const [hasMultipleLines, setHasMultipleLines] = useState(false);
  const [isManuallyExpanded, setIsManuallyExpanded] = useState(false);
  const [isManualTransitioning, setIsManualTransitioning] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const observedWidthRef = useRef(0);
  const manualTransitionTimerRef = useRef<number | null>(null);

  const resizeToContent = useCallback(() => {
    const root = rootRef.current;
    const textarea = textareaRef.current;
    if (!root || !textarea) return;

    root.style.removeProperty("--chat-text-input-height");
    if (isManuallyExpanded) return;

    const minimumHeight = root.getBoundingClientRect().height;
    root.style.setProperty("--chat-text-input-height", "0px");
    const textareaStyles = window.getComputedStyle(textarea);
    const fontSize = pixelValue(textareaStyles.fontSize, 16);
    const lineHeight = pixelValue(textareaStyles.lineHeight, fontSize * 1.5);
    const verticalChrome = pixelValue(textareaStyles.paddingTop)
      + pixelValue(textareaStyles.paddingBottom)
      + pixelValue(textareaStyles.borderTopWidth)
      + pixelValue(textareaStyles.borderBottomWidth);
    const maximumHeight = lineHeight * MAX_AUTO_ROWS + verticalChrome;
    const contentHeight = textarea.scrollHeight;
    const singleLineHeight = lineHeight + verticalChrome;
    setHasMultipleLines(
      value.includes("\n") || contentHeight > singleLineHeight + 1,
    );
    const nextHeight = Math.min(
      Math.max(contentHeight, minimumHeight),
      maximumHeight,
    );

    root.style.setProperty("--chat-text-input-height", `${nextHeight}px`);
  }, [isManuallyExpanded, value]);

  useLayoutEffect(() => {
    resizeToContent();
  }, [appearance, resizeToContent, rows, size, value]);

  useEffect(() => {
    const inputRoot = rootRef.current?.parentElement;
    const handleResize = () => resizeToContent();
    const resizeObserver = inputRoot === null || typeof ResizeObserver === "undefined"
      ? null
      : new ResizeObserver(([entry]) => {
        if (!entry || entry.contentRect.width === observedWidthRef.current) return;
        observedWidthRef.current = entry.contentRect.width;
        resizeToContent();
      });

    resizeObserver?.observe(inputRoot as Element);
    window.addEventListener("resize", handleResize);
    return () => {
      resizeObserver?.disconnect();
      window.removeEventListener("resize", handleResize);
    };
  }, [resizeToContent]);

  useEffect(() => () => {
    if (manualTransitionTimerRef.current !== null) {
      window.clearTimeout(manualTransitionTimerRef.current);
    }
  }, []);

  const submitOnEnter = (event: KeyboardEvent<HTMLElement>) => {
    if (disabled || event.key !== "Enter" || event.shiftKey || event.nativeEvent.isComposing) {
      return;
    }

    event.preventDefault();
    onSend();
  };

  const toggleExpanded = () => {
    if (manualTransitionTimerRef.current !== null) {
      window.clearTimeout(manualTransitionTimerRef.current);
    }
    setIsManualTransitioning(true);
    setIsManuallyExpanded((expanded) => !expanded);
    manualTransitionTimerRef.current = window.setTimeout(() => {
      setIsManualTransitioning(false);
      manualTransitionTimerRef.current = null;
    }, MANUAL_TRANSITION_DURATION_MS);
    textareaRef.current?.focus({ preventScroll: true });
  };

  return (
    <div
      className={styles.root}
      data-manually-expanded={isManuallyExpanded}
      data-manual-transitioning={isManualTransitioning}
      data-ui="chat-text-input"
    >
      <ScrollArea
        className={`${styles.scrollArea} ${styles[size]}`}
        data-appearance={appearance}
        data-max-auto-rows={MAX_AUTO_ROWS}
        data-ui="chat-text-input-scroll-area"
        orientation="vertical"
        ref={rootRef}
        trackPlacement="inset"
        viewportAs="textarea"
        viewportClassName={`${styles.input} ${styles[appearance]} ${styles[size]}`}
        viewportProps={{
          disabled,
          onChange,
          onKeyDown: submitOnEnter,
          placeholder,
          rows,
          value,
        }}
        viewportRef={(node) => {
          textareaRef.current = node as HTMLTextAreaElement | null;
        }}
      />
      {hasMultipleLines || isManuallyExpanded ? (
        <button
          aria-expanded={isManuallyExpanded}
          aria-label={isManuallyExpanded ? "Свернуть поле ввода" : "Развернуть поле ввода"}
          className={styles.expandButton}
          data-state={isManuallyExpanded ? "expanded" : "collapsed"}
          disabled={disabled}
          onClick={toggleExpanded}
          onMouseDown={(event) => event.preventDefault()}
          type="button"
        >
          <svg aria-hidden="true" className={styles.expandIcon} viewBox="0 0 24 24">
            <path
              className={styles.expandGlyph}
              d="M15 3h6v6M9 21H3v-6m18-12-7 7M3 21l7-7"
            />
            <path
              className={styles.collapseGlyph}
              d="M9 3v6H3m18 6h-6v6M3 9l6-6m6 18 6-6"
            />
          </svg>
        </button>
      ) : null}
    </div>
  );
}
