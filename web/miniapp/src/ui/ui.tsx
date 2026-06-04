// src/ui/ui.tsx
export function Spinner({ size = 22 }: { size?: number }) {
  const style = { width: size, height: size };
  return <span className="spinner" style={style} />;
}

export function Avatar({ src, fallback }: { src?: string | null; fallback: string }) {
  if (src) return <img className="avatar" src={src} alt="" />;
  return <span className="avatar avatar--fallback">{fallback}</span>;
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
