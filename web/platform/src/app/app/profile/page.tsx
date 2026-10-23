import { ProfileWorkspace } from "@/features/account/ProfileWorkspace/ProfileWorkspace";
import { loadAuthMethods } from "@/lib/auth/methods.server";
import { isLocalWorkspacePreviewEnabled } from "@/features/session/local-workspace-preview";
import { getLegalCatalog } from "@/features/legal/catalog.server";

export default async function ProfilePage({ searchParams }: { searchParams: Promise<Record<string, string | string[] | undefined>> }) {
  const status = (await searchParams).oauth;
  return <ProfileWorkspace legalMetadata={getLegalCatalog().metadata} methods={await loadAuthMethods()} preview={isLocalWorkspacePreviewEnabled()} oauthStatus={status === "linked" || status === "failed" ? status : undefined} />;
}
