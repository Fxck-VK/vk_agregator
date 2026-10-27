import type { LoginSearchParams } from "@/features/auth/login-page.server";
import { AuthEntry } from "./AuthEntry";

export default async function LoginPage({ searchParams }: { searchParams?: LoginSearchParams } = {}) {
  return AuthEntry({searchParams});
}
