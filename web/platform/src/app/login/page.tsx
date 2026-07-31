import { cookies } from "next/headers";

import { LoginForm } from "@/features/auth/LoginForm/LoginForm";
import { safeReturnPath } from "@/lib/auth/return-path";

import styles from "./page.module.css";

export default async function LoginPage() {
  const returnTo = safeReturnPath((await cookies()).get("__Host-nh-return-to")?.value ?? "");

  return (
    <main className={styles.page}>
      <LoginForm {...(returnTo ? { returnTo } : {})} />
    </main>
  );
}
