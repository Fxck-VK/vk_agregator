import { z } from "zod";

export const oauthProviders = ["google", "apple", "vk", "telegram"] as const;
export type OAuthProvider = typeof oauthProviders[number];
export const providerNames: Record<OAuthProvider, string> = { google: "Google", apple: "Apple", vk: "VK", telegram: "Telegram" };
const disabledUIOAuthProviders = new Set<OAuthProvider>(["google"]);
export const authMethodsSchema = z.object({
  registration: z.boolean().default(false),
  password: z.boolean(), recovery: z.boolean(), email_link: z.boolean(), phone_link: z.boolean(),
  providers: z.array(z.enum(oauthProviders)),
}).strict();
export type AuthMethods = z.infer<typeof authMethodsSchema>;
export const defaultAuthMethods: AuthMethods = { registration: false, password: true, recovery: false, email_link: false, phone_link: false, providers: [] };
export const previewAuthMethods: AuthMethods = { registration: true, password: true, recovery: true, email_link: true, phone_link: true, providers: [...oauthProviders] };

export function visibleOAuthProvidersForUI(providers: readonly OAuthProvider[]): OAuthProvider[] {
  return providers.filter((provider) => !disabledUIOAuthProviders.has(provider));
}

export function safeAuthorizationURL(value: unknown, provider: OAuthProvider): string | null {
  if (typeof value !== "string") return null;
  const origins: Record<OAuthProvider, string> = { google: "https://accounts.google.com", apple: "https://appleid.apple.com", vk: "https://id.vk.ru", telegram: "https://oauth.telegram.org" };
  const paths: Record<OAuthProvider, string> = { google: "/o/oauth2/v2/auth", apple: "/auth/authorize", vk: "/authorize", telegram: "/auth" };
  try { const url = new URL(value); return url.origin === origins[provider] && url.pathname === paths[provider] && !url.username && !url.password && !url.hash ? value : null; } catch { return null; }
}
