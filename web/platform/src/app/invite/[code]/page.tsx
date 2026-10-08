import { notFound } from "@/i18n/server";
import { InviteEntry } from "@/features/referrals/InviteEntry";
import styles from "../../login/page.module.css";

export const metadata = { robots: { index: false, follow: false } };
export default async function InvitePage({ params }: { params: Promise<{ code: string }> }) {
  const { code } = await params;
  if (!/^[A-Za-z0-9_-]{4,64}$/.test(code)) notFound();
  return <main className={styles.page}><InviteEntry code={code} /></main>;
}
