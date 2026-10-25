import Link from "@/i18n/Link";
import { LandingArrow } from "./LandingPrimitives";
import styles from "./LandingPlanCards.module.css";

type Plan = { readonly label: string; readonly title: string; readonly description: string; readonly points: readonly string[]; readonly action: string; readonly href: string };
/** The shared three-card layout used by the matching T06/T07 reference sections. */
export function LandingPlanCards({ id = "plans", title, description, items }: { id?: string; title: string; description: string; items: readonly Plan[] }) {
  return <section id={id} className={styles.plans}><div className={styles.heading}><h2>{title}</h2><p>{description}</p></div><div className={styles.grid}>{items.map((plan,index)=><article key={plan.title} className={styles.card} data-featured={index===1||undefined} data-plan-card><span className={styles.label}>{plan.label}</span><h3>{plan.title}</h3><p>{plan.description}</p><Link href={plan.href} className={styles.action}>{plan.action}<LandingArrow /></Link><ul>{plan.points.map(point=><li key={point}><span aria-hidden="true">✓</span>{point}</li>)}</ul></article>)}</div></section>;
}
