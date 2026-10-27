import { LoginForm } from "@/features/auth/LoginForm/LoginForm";
import type { AuthScreen } from "@/features/auth/CanonicalLogin";
import { LoginWindow } from "@/features/auth/LoginWindow/LoginWindow";
import { loadLoginPageProps, type LoginSearchParams } from "@/features/auth/login-page.server";
import styles from "./page.module.css";

export async function AuthEntry({screen,searchParams}:{screen?:AuthScreen;searchParams?:LoginSearchParams}) {
 const props=await loadLoginPageProps(searchParams);
 const query=await searchParams;
 const initialScreen=screen??(props.initialMode==="register"?"register":query?.step==="password"?"password":"choice");
 return <main className={styles.page}><LoginWindow><LoginForm {...props} initialScreen={initialScreen} appearance="minimal" navigationEnabled /></LoginWindow></main>;
}
