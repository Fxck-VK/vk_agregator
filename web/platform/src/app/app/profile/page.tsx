import { ProfileWorkspace } from "@/features/account/ProfileWorkspace/ProfileWorkspace";
import { loadAuthMethods } from "@/lib/auth/methods.server";
import { isLocalWorkspacePreviewEnabled } from "@/features/session/local-workspace-preview";

export default async function ProfilePage({ searchParams }: { searchParams: Promise<Record<string, string | string[] | undefined>> }) {
  const status = (await searchParams).oauth;
  return <ProfileWorkspace methods={await loadAuthMethods()} preview={isLocalWorkspacePreviewEnabled()} oauthStatus={status === "linked" || status === "failed" ? status : undefined} />;
}
