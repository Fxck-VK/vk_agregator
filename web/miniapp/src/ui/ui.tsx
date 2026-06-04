// src/ui/ui.tsx
import { type ButtonHTMLAttributes, type ReactNode } from "react";
import { statusKind, statusLabel } from "../api/client";

export function Screen({
  title, subtitle, right, children,
}: { title: string; subtitle?: string; right?: ReactNode; children: ReactNode }) {
  return (
    <section className="screen">
      <header className="screen__head">
        <div>
          <h1 className="screen__title">{title}</h1>
          {subtitle && <p className="screen__sub">{subtitle}</p>}
        </div>
        {right && <div className="screen__right">{right}</div>}
      </header>
      <div className="screen__body">{children}</div>
    </section>
  );
}

export function Card({ children, onClick }: { children: ReactNode; onClick?: () => void }) {
  return (
    <div
      className={"card" + (onClick ? " card--tap" : "")}
      onClick={onClick}
      role={onClick ? "button" : undefined}
    >
      {children}
    </div>
  );
}

export function Button({
  children, variant = "primary", ...rest
}: ButtonHTMLAttributes<HTMLButtonElement> & { variant?: "primary" | "ghost" }) {
  return (
    <button className={`btn btn--${variant}`} {...rest}>
      {children}
    </button>
  );
}

export function Spinner({ size = 20 }: { size?: number }) {
  const style = { width: size, height: size };
  return <span className="spinner" style={style} />;
}

export function StatusPill({ status }: { status: string }) {
  return <span className={`pill pill--${statusKind(status)}`}>{statusLabel(status)}</span>;
}