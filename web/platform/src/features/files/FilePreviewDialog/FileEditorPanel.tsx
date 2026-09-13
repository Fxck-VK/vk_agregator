"use client";

/* eslint-disable @next/next/no-img-element */

import Image from "next/image";
import {
  useEffect,
  useId,
  useRef,
  useState,
  type PointerEvent as ReactPointerEvent,
} from "react";

import { InputSurface } from "@/components/ui/InputSurface/InputSurface";
import { RangeSlider } from "@/components/ui/RangeSlider/RangeSlider";
import { ScrollArea } from "@/components/ui/ScrollArea/ScrollArea";
import { ImageAspectRatioSelector } from "@/features/image-generation/ImageAspectRatioSelector/ImageAspectRatioSelector";
import { ImageQualitySelector } from "@/features/image-generation/ImageQualitySelector/ImageQualitySelector";

import { getFileActionModels } from "./file-action-models";
import styles from "./FileEditorPanel.module.css";
import { FileTaskModelSelector } from "./FileTaskModelSelector";

type EditPoint = { x: number; y: number };
type EditTool = "brush" | "eraser" | "lasso";
type EditStroke = {
  brushSize: number;
  id: number;
  points: readonly EditPoint[];
  tool: EditTool;
};
type EditStrokeInput = Omit<EditStroke, "id">;
type EditView = { zoom: number; offset: EditPoint; isPanMode: boolean };

const initialEditView: EditView = { zoom: 1, offset: { x: 0, y: 0 }, isPanMode: false };
const editZoomLevels = [1, 1.2, 1.5, 2, 3, 4, 5];

const brushSizeMin = 8;
const brushSizeMax = 80;
const editModels = getFileActionModels("edit");
const editQualityOptions = ["1K", "2K", "4K"] as const;

export type FileEditorController = {
  addStroke: (stroke: EditStrokeInput) => void;
  aspectRatio: string;
  brushSize: number;
  canClear: boolean;
  canRedo: boolean;
  canUndo: boolean;
  clear: () => void;
  imageQuality: string;
  prompt: string;
  redo: () => void;
  reset: () => void;
  setAspectRatio: (ratio: string) => void;
  setBrushSize: (size: number) => void;
  setImageQuality: (quality: string) => void;
  setPrompt: (prompt: string) => void;
  setView: (view: EditView) => void;
  strokes: readonly EditStroke[];
  toggleTool: (tool: EditTool) => void;
  tool: EditTool | null;
  undo: () => void;
  view: EditView;
};

function clamp(value: number, minimum: number, maximum: number) {
  return Math.min(maximum, Math.max(minimum, value));
}

export function useFileEditorController(): FileEditorController {
  const [aspectRatio, setAspectRatio] = useState("9:16");
  const [brushSize, setBrushSize] = useState(32);
  const [imageQuality, setImageQuality] = useState("2K");
  const [prompt, setPrompt] = useState("");
  const [redoStack, setRedoStack] = useState<readonly EditStroke[]>([]);
  const [strokes, setStrokes] = useState<readonly EditStroke[]>([]);
  const [tool, setTool] = useState<EditTool | null>("brush");
  const [view, setView] = useState<EditView>(initialEditView);
  const nextStrokeId = useRef(0);

  const addStroke = (stroke: EditStrokeInput) => {
    setStrokes((current) => [...current, { ...stroke, id: nextStrokeId.current++ }]);
    setRedoStack([]);
  };

  const clear = () => {
    setStrokes([]);
    setRedoStack([]);
  };

  const undo = () => {
    const lastStroke = strokes.at(-1);
    if (!lastStroke) return;
    setStrokes(strokes.slice(0, -1));
    setRedoStack((current) => [lastStroke, ...current]);
  };

  const redo = () => {
    const nextStroke = redoStack[0];
    if (!nextStroke) return;
    setStrokes((current) => [...current, nextStroke]);
    setRedoStack(redoStack.slice(1));
  };

  const reset = () => {
    setAspectRatio("9:16");
    setBrushSize(32);
    setImageQuality("2K");
    setPrompt("");
    setRedoStack([]);
    setStrokes([]);
    setTool("brush");
    setView(initialEditView);
  };

  const toggleTool = (nextTool: EditTool) => {
    setTool((currentTool) => (currentTool === nextTool && !view.isPanMode ? null : nextTool));
    setView((current) => ({ ...current, isPanMode: false }));
  };

  return {
    addStroke,
    aspectRatio,
    brushSize,
    canClear: strokes.length > 0,
    canRedo: redoStack.length > 0,
    canUndo: strokes.length > 0,
    clear,
    imageQuality,
    prompt,
    redo,
    reset,
    setAspectRatio,
    setBrushSize,
    setImageQuality,
    setPrompt,
    setView,
    strokes,
    toggleTool,
    tool,
    undo,
    view,
  };
}

function HistoryIcon({ direction }: Readonly<{ direction: "left" | "right" }>) {
  return (
    <svg aria-hidden="true" className={styles.controlIcon} viewBox="0 0 24 24">
      <path d={direction === "left" ? "M9 7 4 12l5 5" : "m15 7 5 5-5 5"} />
      <path d={direction === "left" ? "M5 12h8a6 6 0 0 1 6 6" : "M19 12h-8a6 6 0 0 0-6 6"} />
    </svg>
  );
}

function EraserIcon() {
  return (
    <svg aria-hidden="true" className={styles.controlIcon} viewBox="0 0 24 24">
      <path d="m7 19-4-4 9-9a2.8 2.8 0 0 1 4 0l2 2a2.8 2.8 0 0 1 0 4l-7 7H7Z" />
      <path d="m9 9 6 6M11 19h9" />
    </svg>
  );
}

function CreditStar() {
  return (
    <Image
      alt=""
      aria-hidden="true"
      className={styles.creditStar}
      height={18}
      src="/assets/icons/ui/star-white.svg"
      unoptimized
      width={18}
    />
  );
}

export function FileEditorPanel({
  controller,
}: Readonly<{ controller: FileEditorController }>) {
  const [selectedModelId, setSelectedModelId] = useState<string>(editModels[0].id);
  const selectedModel = editModels.find((model) => model.id === selectedModelId) ?? editModels[0];
  const isBrushSizeVisible = controller.tool !== "lasso";

  return (
    <section aria-label="Настройки редактирования" className={styles.panel}>
      <header className={styles.title}>
        <Image
          alt=""
          aria-hidden="true"
          height={20}
          src="/assets/icons/ui/edit-white.svg"
          unoptimized
          width={20}
        />
        <h2>Редактировать</h2>
      </header>

      <ScrollArea
        className={styles.bodyScroll}
        viewportClassName={styles.bodyViewport}
        viewportProps={{ "aria-label": "Параметры редактирования" }}
      >
        <div className={styles.bodyContent}>
          <div className={styles.historyBar}>
            <button
              aria-label="Ластик"
              aria-pressed={!controller.view.isPanMode && controller.tool === "eraser"}
              className={styles.iconButton}
              data-active={!controller.view.isPanMode && controller.tool === "eraser"}
              onClick={() => controller.toggleTool("eraser")}
              type="button"
            >
              <EraserIcon />
            </button>
            <div className={styles.historyActions}>
              <button
                aria-label="Очистить выделение"
                className={styles.clearButton}
                disabled={!controller.canClear}
                onClick={controller.clear}
                type="button"
              >
                Очистить
              </button>
              <button
                aria-label="Отменить выделение"
                className={styles.iconButton}
                disabled={!controller.canUndo}
                onClick={controller.undo}
                type="button"
              >
                <HistoryIcon direction="left" />
              </button>
              <button
                aria-label="Повторить выделение"
                className={styles.iconButton}
                disabled={!controller.canRedo}
                onClick={controller.redo}
                type="button"
              >
                <HistoryIcon direction="right" />
              </button>
            </div>
          </div>

          <div>
            <section className={styles.controlSection}>
              <h3>Выделение</h3>
              <div className={styles.selectionModes}>
                <button
                  aria-pressed={!controller.view.isPanMode && controller.tool === "brush"}
                  data-active={!controller.view.isPanMode && controller.tool === "brush"}
                  onClick={() => controller.toggleTool("brush")}
                  type="button"
                >
                  <span
                    aria-hidden="true"
                    className={`${styles.selectionToolIcon} ${styles.brushIcon}`}
                  />
                  Кисть
                </button>
                <button
                  aria-pressed={!controller.view.isPanMode && controller.tool === "lasso"}
                  data-active={!controller.view.isPanMode && controller.tool === "lasso"}
                  onClick={() => controller.toggleTool("lasso")}
                  type="button"
                >
                  <span
                    aria-hidden="true"
                    className={`${styles.selectionToolIcon} ${styles.lassoIcon}`}
                  />
                  Лассо
                </button>
              </div>
            </section>

            <div
              aria-hidden={!isBrushSizeVisible}
              className={styles.brushSizeReveal}
              data-open={isBrushSizeVisible}
              inert={!isBrushSizeVisible}
            >
              <div className={styles.brushSizeClip}>
                <label className={`${styles.field} ${styles.brushSizeField}`}>
                  <span>Размер кисти</span>
                  <RangeSlider
                    aria-label="Размер кисти"
                    disabled={!isBrushSizeVisible}
                    max={brushSizeMax}
                    min={brushSizeMin}
                    onValueChange={controller.setBrushSize}
                    value={controller.brushSize}
                  />
                </label>
              </div>
            </div>
          </div>

          <label className={styles.field}>
            <span>Промпт</span>
            <InputSurface className={styles.promptSurface}>
              <ScrollArea
                className={styles.promptScroll}
                viewportAs="textarea"
                viewportProps={{
                  "aria-label": "Промпт редактирования",
                  onChange: (event) => controller.setPrompt(event.target.value),
                  placeholder: "Опиши, что нужно изменить",
                  value: controller.prompt,
                }}
              />
            </InputSurface>
          </label>

          <div className={`${styles.field} ${styles.modelField}`}>
            <span>Модель</span>
            <FileTaskModelSelector
              onSelect={setSelectedModelId}
              selectedModelId={selectedModelId}
              task="edit"
            />
          </div>

          <section className={styles.controlSection}>
            <h3>Настройки генерации</h3>
            <div className={styles.generationSettings}>
              <ImageAspectRatioSelector
                disabled={false}
                onChange={controller.setAspectRatio}
                portalLayer={170}
                value={controller.aspectRatio}
              />
              <ImageQualitySelector
                disabled={false}
                label="Разрешение"
                onChange={controller.setImageQuality}
                options={editQualityOptions}
                portalLayer={170}
                value={controller.imageQuality}
              />
            </div>
          </section>

          <section className={styles.howItWorks}>
            <h3>Как работает</h3>
            <div className={styles.howCard}>
              <Image
                alt=""
                aria-hidden="true"
                height={293}
                src="/assets/images/inspiration/spider-man-selfie.png"
                unoptimized
                width={520}
              />
              <div className={styles.howCardCopy}>
                <strong>Изменяет часть изображения</strong>
                <p>Выделяет нужную область и позволяет описать, что в ней изменить.</p>
              </div>
            </div>
          </section>
        </div>
      </ScrollArea>

      <button
        aria-label={`Редактировать за ${selectedModel.cost} звёзд`}
        className={styles.submitButton}
        type="button"
      >
        Редактировать за {selectedModel.cost}
        <CreditStar />
      </button>
    </section>
  );
}

type FileEditPreviewProps = {
  alt: string;
  controller: FileEditorController;
  imageClassName: string;
  src: string;
};

function getEditPoint(event: ReactPointerEvent<HTMLDivElement>): EditPoint {
  const bounds = event.currentTarget.getBoundingClientRect();
  const width = bounds.width || 100;
  const height = bounds.height || 100;
  return {
    x: clamp(((event.clientX - bounds.left) / width) * 100, 0, 100),
    y: clamp(((event.clientY - bounds.top) / height) * 100, 0, 100),
  };
}

function clampPanOffset(offset: EditPoint, zoom: number): EditPoint {
  const limit = Math.round((zoom - 1) * 50 * 1000) / 1000;
  return { x: clamp(offset.x, -limit, limit), y: clamp(offset.y, -limit, limit) };
}

function renderMaskStroke(
  stroke: EditStroke | EditStrokeInput,
  key: number | string,
  isDraft = false,
) {
  const firstPoint = stroke.points[0];
  const lastPoint = stroke.points.at(-1);
  if (!firstPoint || !lastPoint) return null;
  const points = stroke.points.map((point) => `${point.x},${point.y}`).join(" ");

  if (stroke.tool === "lasso" && isDraft) {
    return (
      <polyline
        data-edit-draft="true"
        data-edit-stroke="true"
        data-edit-tool={stroke.tool}
        fill="none"
        key={key}
        points={points}
        stroke="#fff"
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth="1.5"
        vectorEffect="non-scaling-stroke"
      />
    );
  }

  if (stroke.tool === "lasso") {
    return (
      <polygon
        data-edit-stroke="true"
        data-edit-tool={stroke.tool}
        fill="#fff"
        key={key}
        points={points}
      />
    );
  }

  return (
    <polyline
      data-edit-stroke="true"
      data-edit-tool={stroke.tool}
      fill="none"
      key={key}
      points={points}
      stroke={stroke.tool === "eraser" ? "#000" : "#fff"}
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth={stroke.brushSize}
      vectorEffect="non-scaling-stroke"
    />
  );
}

export function FileEditPreview({
  alt,
  controller,
  imageClassName,
  src,
}: Readonly<FileEditPreviewProps>) {
  const [isBrushSizePreviewVisible, setIsBrushSizePreviewVisible] = useState(false);
  const [draftStroke, setDraftStroke] = useState<EditStrokeInput | null>(null);
  const [isToolCursorVisible, setIsToolCursorVisible] = useState(false);
  const [toolCursor, setToolCursor] = useState<EditPoint>({ x: 0, y: 0 });
  const [isPanning, setIsPanning] = useState(false);
  const panGesture = useRef<{ pointerId: number; start: EditPoint; offset: EditPoint } | null>(null);
  const { zoom, offset, isPanMode } = controller.view;
  const previousBrushSize = useRef(controller.brushSize);
  const selectionMaskId = `file-edit-selection-${useId().replace(/[^a-zA-Z0-9_-]/g, "")}`;
  const usesBrushCursor = !isPanMode && (controller.tool === "brush" || controller.tool === "eraser");

  const imagePoint = (point: EditPoint): EditPoint => ({
    x: clamp((point.x - 50 - offset.x) / zoom + 50, 0, 100),
    y: clamp((point.y - 50 - offset.y) / zoom + 50, 0, 100),
  });

  const changeZoom = (direction: -1 | 1) => {
    const nextIndex = clamp(editZoomLevels.indexOf(zoom) + direction, 0, editZoomLevels.length - 1);
    const nextZoom = editZoomLevels[nextIndex];
    controller.setView(nextZoom === 1
      ? initialEditView
      : { ...controller.view, zoom: nextZoom, offset: clampPanOffset(offset, nextZoom) });
    setIsToolCursorVisible(false);
  };

  const moveImage = (event: ReactPointerEvent<HTMLDivElement>) => {
    const gesture = panGesture.current;
    if (!gesture || gesture.pointerId !== event.pointerId) return;
    const bounds = event.currentTarget.getBoundingClientRect();
    controller.setView({
      ...controller.view,
      offset: clampPanOffset({
        x: gesture.offset.x + (event.clientX - gesture.start.x) / (bounds.width || 100) * 100,
        y: gesture.offset.y + (event.clientY - gesture.start.y) / (bounds.height || 100) * 100,
      }, zoom),
    });
  };

  const cancelGesture = () => {
    panGesture.current = null;
    setIsPanning(false);
    setDraftStroke(null);
    setIsToolCursorVisible(false);
  };

  useEffect(() => {
    if (previousBrushSize.current === controller.brushSize) return;
    previousBrushSize.current = controller.brushSize;
    setIsBrushSizePreviewVisible(true);
    const hidePreview = window.setTimeout(() => setIsBrushSizePreviewVisible(false), 700);
    return () => window.clearTimeout(hidePreview);
  }, [controller.brushSize]);

  const startDrawing = (event: ReactPointerEvent<HTMLDivElement>) => {
    if (event.button !== 0 || panGesture.current) return;
    if (isPanMode) {
      event.preventDefault();
      panGesture.current = {
        pointerId: event.pointerId,
        start: { x: event.clientX, y: event.clientY },
        offset,
      };
      setIsPanning(true);
      setIsToolCursorVisible(false);
      event.currentTarget.setPointerCapture?.(event.pointerId);
      return;
    }
    if (event.button !== 0 || !controller.tool) return;
    event.preventDefault();
    const point = getEditPoint(event);
    if (usesBrushCursor) {
      setToolCursor(point);
      setIsToolCursorVisible(true);
    } else {
      setIsToolCursorVisible(false);
    }
    event.currentTarget.setPointerCapture?.(event.pointerId);
    setDraftStroke({
      brushSize: controller.brushSize / zoom,
      points: [imagePoint(point)],
      tool: controller.tool,
    });
  };

  const continueDrawing = (event: ReactPointerEvent<HTMLDivElement>) => {
    if (isPanMode) {
      moveImage(event);
      return;
    }
    if (!controller.tool) {
      setIsToolCursorVisible(false);
      return;
    }
    const point = getEditPoint(event);
    if (usesBrushCursor) {
      setToolCursor(point);
      setIsToolCursorVisible(true);
    } else {
      setIsToolCursorVisible(false);
    }
    if (!draftStroke) return;
    setDraftStroke((current) => {
      if (!current) return current;
      return {
        ...current,
        points: [...current.points, imagePoint(point)],
      };
    });
  };

  const finishDrawing = (event: ReactPointerEvent<HTMLDivElement>) => {
    if (panGesture.current) {
      if (panGesture.current.pointerId !== event.pointerId) return;
      moveImage(event);
      panGesture.current = null;
      setIsPanning(false);
      event.currentTarget.releasePointerCapture?.(event.pointerId);
      return;
    }
    if (!draftStroke) return;
    const point = getEditPoint(event);
    const completedStroke = {
      ...draftStroke,
      points: [...draftStroke.points, imagePoint(point)],
    };
    controller.addStroke(completedStroke);
    setDraftStroke(null);
    event.currentTarget.releasePointerCapture?.(event.pointerId);
  };

  return (
    <div
      aria-label="Область редактирования изображения"
      className={styles.editSurface}
      data-panning={isPanning}
      data-tool={isPanMode ? "pan" : controller.tool ?? "none"}
      onLostPointerCapture={cancelGesture}
      onPointerCancel={cancelGesture}
      onPointerDown={startDrawing}
      onPointerLeave={() => setIsToolCursorVisible(false)}
      onPointerMove={continueDrawing}
      onPointerUp={finishDrawing}
      role="application"
    >
      <div
        className={styles.editCanvas}
        style={{ transform: `translate(${offset.x}%, ${offset.y}%) scale(${zoom})` }}
      >
        <img alt={alt} className={`${imageClassName} ${styles.editImage}`} src={src} />
        <svg
          aria-hidden="true"
          className={styles.editMask}
          data-testid="file-edit-mask"
          preserveAspectRatio="none"
          viewBox="0 0 100 100"
        >
          <defs>
            <mask
              height="100"
              id={selectionMaskId}
              maskUnits="userSpaceOnUse"
              style={{ maskType: "luminance" }}
              width="100"
              x="0"
              y="0"
            >
              <rect fill="#000" height="100" width="100" x="0" y="0" />
              {controller.strokes.map((stroke) => renderMaskStroke(stroke, stroke.id))}
              {draftStroke ? renderMaskStroke(draftStroke, "draft", true) : null}
            </mask>
          </defs>
          <rect
            fill="rgb(186 106 255 / 56%)"
            height="100"
            mask={`url(#${selectionMaskId})`}
            width="100"
            x="0"
            y="0"
          />
        </svg>
      </div>
      <div
        aria-hidden="true"
        className={styles.toolCursor}
        data-testid="file-edit-tool-cursor"
        data-tool={controller.tool ?? "none"}
        data-visible={isToolCursorVisible && usesBrushCursor}
        style={{
          height: `${controller.brushSize}px`,
          left: `${toolCursor.x}%`,
          top: `${toolCursor.y}%`,
          width: `${controller.brushSize}px`,
        }}
      />
      <div
        aria-hidden="true"
        className={`${styles.toolCursor} ${styles.brushSizePreview}`}
        data-testid="file-edit-brush-size-preview"
        data-visible={isBrushSizePreviewVisible && !isPanMode && controller.tool !== "lasso"}
        style={{
          height: `${controller.brushSize}px`,
          left: "50%",
          top: "50%",
          width: `${controller.brushSize}px`,
        }}
      />
      <div
        className={styles.zoomControls}
        data-zoomed={zoom > 1}
        onPointerDown={(event) => event.stopPropagation()}
      >
        <button
          aria-hidden={zoom <= 1}
          aria-label="Перемещать изображение"
          aria-pressed={isPanMode}
          className={styles.zoomPanButton}
          disabled={zoom <= 1}
          onClick={() => {
            controller.setView({ ...controller.view, isPanMode: !isPanMode });
            setIsToolCursorVisible(false);
          }}
          tabIndex={zoom > 1 ? 0 : -1}
          title="Перемещать изображение"
          type="button"
        >
          <svg aria-hidden="true" className={styles.controlIcon} viewBox="0 0 24 24">
            <path d="M12 3v18M3 12h18M8 7l4-4 4 4M8 17l4 4 4-4M7 8l-4 4 4 4M17 8l4 4-4 4" />
          </svg>
        </button>
        <div
          aria-label="Масштаб изображения"
          className={styles.zoomActions}
          role="group"
        >
          <button
            aria-label="Уменьшить масштаб"
            disabled={zoom <= 1}
            onClick={() => changeZoom(-1)}
            type="button"
          >
            −
          </button>
          <output aria-live="polite">{zoom}×</output>
          <button
            aria-label="Увеличить масштаб"
            disabled={zoom >= editZoomLevels[editZoomLevels.length - 1]}
            onClick={() => changeZoom(1)}
            type="button"
          >
            +
          </button>
        </div>
      </div>
    </div>
  );
}
