import Link from '@/i18n/Link';
import { getRequestDictionary } from '@/i18n/server';
import styles from './page.module.css';

export default async function LandingVariantsPage() {
  const { landingVariants: copy } = await getRequestDictionary();
  return <main className={styles.page}>
    <h1>{copy.title}</h1>
    <p>{copy.description}</p>
    <div className={styles.cards}>
      <Link href="/landing-variants/original"><h2>{copy.original}</h2><p>{copy.originalDescription}</p><span>{copy.open}</span></Link>
      <Link href="/landing-variants/friend"><h2>{copy.friend}</h2><p>{copy.friendDescription}</p><span>{copy.open}</span></Link>
    </div>
    <p><Link href="/app">{copy.back}</Link></p>
  </main>;
}
