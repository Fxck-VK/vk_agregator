// src/ui/ui.tsx
import { useEffect, useState } from "react";

export function Spinner({ size = 22 }: { size?: number }) {
  const style = { width: size, height: size };
  return <span className="spinner" style={style} role="status" aria-label="Загрузка" />;
}

export function Avatar({
  src,
  fallback,
  className = "",
}: {
  src?: string | null;
  fallback: string;
  className?: string;
}) {
  const classes = "avatar" + (className ? " " + className : "");
  if (src) {
    return (
      <span className={classes}>
        <img className="avatar__img" src={src} alt="" />
      </span>
    );
  }
  return <span className={classes + " avatar--fallback"}>{fallback}</span>;
}

export function TypingDots() {
  return (
    <span className="typing">
      <span />
      <span />
      <span />
    </span>
  );
}

const RESPONDING_SUFFIXES = [".", "..", "...", ". ..", ". .. ...", ". .. ... ."];

export function RespondingLabel() {
  const [index, setIndex] = useState(0);

  useEffect(() => {
    const timer = window.setInterval(() => {
      setIndex((value) => (value + 1) % RESPONDING_SUFFIXES.length);
    }, 520);
    return () => window.clearInterval(timer);
  }, []);

  return (
    <div className="bubble__status bubble__status--responding" aria-live="polite">
      отвечает{RESPONDING_SUFFIXES[index]}
    </div>
  );
}
