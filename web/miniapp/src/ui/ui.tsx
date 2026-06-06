// src/ui/ui.tsx
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
