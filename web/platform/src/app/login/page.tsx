import { cookies } from "next/headers";
import { redirect } from "next/navigation";

import { LoginForm } from "@/features/auth/LoginForm/LoginForm";
import { safeReturnPath } from "@/lib/auth/return-path";
import { webServerFetch } from "@/lib/web-api/server";

import styles from "./page.module.css";

type LoginPageProps = {
  searchParams?: Promise<{ refresh_failed?: string | string[] }>;
};

export default async function LoginPage({ searchParams }: LoginPageProps = {}) {
  const cookieStore = await cookies();
  const refreshFailed = (await searchParams)?.refresh_failed === "1";
  const returnTo = safeReturnPath(cookieStore.get("__Host-nh-return-to")?.value ?? "");
  let sessionStatus: number | null = null;

  try {
    sessionStatus = (await webServerFetch("/web/v1/me")).status;
  } catch {
    // The login form remains available when the session check is temporarily unavailable.
  }

  if (
    sessionStatus === 200
    || (!refreshFailed && sessionStatus === 401 && cookieStore.has("nh_refresh"))
  ) {
    redirect(returnTo ?? "/app");
  }

  return (
    <main className={styles.page}>
      <LoginForm {...(returnTo ? { returnTo } : {})} />
    </main>
  );
}
