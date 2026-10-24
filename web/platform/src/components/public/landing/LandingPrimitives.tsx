import type { ReactNode } from "react";
import { PageContainer } from "@/components/public/PageContainer/PageContainer";
import { PrimaryButton } from "@/components/public/PrimaryButton/PrimaryButton";
import type { LinkProps } from "@/i18n/Link";
import styles from "./LandingPrimitives.module.css";

export function LandingArrow({ direction = "right" }: { direction?: "right" | "left" }) {
  return <svg aria-hidden="true" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" style={direction === "left" ? { rotate: "180deg" } : undefined}><path d="M4 12h15m-6-6 6 6-6 6" /></svg>;
}

export function LandingLink({ children, tone = "dark", className, ...props }: LinkProps & { children: ReactNode; tone?: "dark" | "light" | "outline"; className?: string }) {
  return <PrimaryButton {...props} className={`${styles.action} ${className ?? ""}`} data-tone={tone}>{children}</PrimaryButton>;
}

export function LandingSection({ id, title, description, action, children, className, align = "center" }: {
  id: string; title: string; description?: string; action?: ReactNode; children: ReactNode; className?: string; align?: "center" | "start";
}) {
  return <section id={id} className={`${styles.section} ${className ?? ""}`} aria-labelledby={`${id}-heading`}>
    <PageContainer size="wide" className={styles.container}>
      <div className={styles.intro} data-align={align}><div><h2 id={`${id}-heading`}>{title}</h2>{description ? <p>{description}</p> : null}</div>{action}</div>
      {children}
    </PageContainer>
  </section>;
}

export function LandingToolIcon({ name }: { name: string }) {
  return <svg aria-hidden="true" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
    {name === "image" ? <><rect x="3" y="3" width="18" height="18" rx="4" /><circle cx="8" cy="8" r="1.5" /><path d="m3 17 5-5 4 4 4-6 5 6" /></> : name === "video" ? <><rect x="3" y="5" width="18" height="14" rx="4" /><path d="m10 9 5 3-5 3Z" /></> : name === "audio" ? <><path d="M9 17V5l11-2v12M9 8l11-2" /><ellipse cx="6" cy="18" rx="3" ry="2.5" /><ellipse cx="17" cy="16" rx="3" ry="2.5" /></> : name === "voice" ? <><rect x="9" y="2" width="6" height="13" rx="3" /><path d="M5 10v2a7 7 0 0 0 14 0v-2m-7 9v3m-4 0h8" /></> : name === "text" ? <><path d="M4 5h16M12 5v15m-4 0h8" /></> : <path d="m12 2 2.7 7.3L22 12l-7.3 2.7L12 22l-2.7-7.3L2 12l7.3-2.7Z" />}
  </svg>;
}
