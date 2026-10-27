import type { LoginSearchParams } from "@/features/auth/login-page.server";
import { AuthEntry } from "../AuthEntry";
export default function Page({searchParams}:{searchParams?:LoginSearchParams}){return AuthEntry({screen:"recover",searchParams});}
