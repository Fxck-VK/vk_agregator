import { LoginForm } from "@/features/auth/LoginForm/LoginForm";
import type { AuthScreen } from "@/features/auth/CanonicalLogin";
import { LoginWindow } from "@/features/auth/LoginWindow/LoginWindow";
import { loadLoginPageProps, type LoginSearchParams } from "@/features/auth/login-page.server";
import styles from "./page.module.css";

export async function AuthEntry({screen,searchParams}:{screen?:AuthScreen;searchParams?:LoginSearchParams}) {
 const props=await loadLoginPageProps(searchParams);
 const query=await searchParams;
 const initialScreen=screen??(props.initialMode==="register"?"register":query?.step==="password"?"password":"choice");
 const formProps={...props,initialScreen,appearance:"minimal" as const,navigationEnabled:true};
 const previewAccount=props.preview&&initialScreen==="choice"?(query?.demo==="provider"?"yandex":query?.demo==="vk"?"vk":undefined):undefined;
 return <main className={styles.page}><LoginWindow><LoginForm {...formProps} previewAccount={previewAccount}/></LoginWindow></main>;
}
