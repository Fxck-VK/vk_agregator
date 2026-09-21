"use client";

/* eslint-disable @next/next/no-img-element */
import { useEffect, useRef, useState, type ComponentPropsWithoutRef, type MouseEventHandler, type ReactNode } from "react";
import Image from "next/image";
import { MediaState } from "@/components/ui/AsyncState/AsyncState";
import { useDictionary } from "@/i18n/LocaleProvider";
import styles from "./MediaImage.module.css";

type Props = ComponentPropsWithoutRef<"img"> & {
  src: string; alt: string; loadingLabel?: string; errorLabel?: string; retryLabel?: string;
  fit?: "natural" | "contain" | "cover"; passive?: boolean; children?: ReactNode;
  optimized?: boolean; priority?: boolean; showLoading?: boolean;
  action?: { label: string; onClick: MouseEventHandler<HTMLButtonElement>; className?: string };
};

export function MediaImage(props: Props) {
  return <ImageAttempt key={props.src} {...props} />;
}

function ImageAttempt({ loadingLabel, errorLabel, retryLabel, action, fit = "natural", passive = false, children, onLoad, onError, style, optimized = false, priority = false, showLoading = true, alt, ...props }: Props) {
  const t = useDictionary();
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");
  const [attempt, setAttempt] = useState(0);
  const [slowAttempt, setSlowAttempt] = useState<number | null>(null);
  const root = useRef<HTMLSpanElement>(null);
  useEffect(() => {
    if (state !== "loading") return;
    let timer: ReturnType<typeof setTimeout> | undefined;
    const begin = () => { timer ??= setTimeout(() => setSlowAttempt(attempt), 8000); };
    const offline = () => { if (timer !== undefined) setState("error"); };
    const observer = props.loading === "lazy" && typeof IntersectionObserver !== "undefined"
      ? new IntersectionObserver(entries => { if (entries.some(entry => entry.isIntersecting)) { begin(); observer?.disconnect(); } }, { rootMargin: "300px" })
      : null;
    if (observer && root.current) observer.observe(root.current); else begin();
    window.addEventListener("offline", offline);
    return () => { clearTimeout(timer); observer?.disconnect(); window.removeEventListener("offline", offline); };
  }, [attempt, props.loading, state]);
  const imageProps = {
    decoding: "async" as const, ...props, alt, style, "data-image-state": state,
    ref: (image: HTMLImageElement | null) => { if (image?.complete && image.naturalWidth > 0) setState("ready"); },
    onLoad: (event: React.SyntheticEvent<HTMLImageElement>) => { setState("ready"); onLoad?.(event); },
    onError: (event: React.SyntheticEvent<HTMLImageElement>) => { setState("error"); onError?.(event); },
  };
  const media = <>{optimized
    ? <Image {...imageProps} key={attempt} alt={alt} width={Number(props.width)} height={Number(props.height)} priority={priority} />
    : <img {...imageProps} key={attempt} alt={alt} />}{children}</>;
  return <span ref={root} className={styles.root} data-ui="media-image" data-state={state} data-fit={fit}
    style={fit === "natural" && state !== "ready" && props.width && props.height ? { aspectRatio: `${props.width} / ${props.height}` } : undefined}>
    {action ? <button type="button" className={action.className ?? styles.open} aria-label={action.label} onClick={action.onClick}>{media}</button> : media}
    {state === "error" || (state === "loading" && showLoading) ? <span className={styles.overlay} data-passive={passive || undefined}
      onPointerDown={event => { if (state === "error" && !passive) event.stopPropagation(); }}>
      <MediaState state={state} compact={passive} label={state === "loading" ? (loadingLabel ?? t.files.previewLoading) : (errorLabel ?? t.files.previewFailure)}
        retry={state === "error" && !passive ? { label: retryLabel ?? t.files.previewRetry, onClick: () => { setState("loading"); setAttempt(value => value + 1); } } : undefined} />
      {state === "loading" && slowAttempt === attempt && !passive ? <span className={styles.slow} role="status">{t.preloading.imageSlow}</span> : null}
    </span> : null}
  </span>;
}
