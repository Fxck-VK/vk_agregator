import { WorkspaceHome } from "@/features/workspace/WorkspaceHome/WorkspaceHome";

export default async function ChatsPage({ searchParams }: { searchParams: Promise<{ model?: string | string[] }> }) {
  const { model } = await searchParams;
  return <WorkspaceHome chatModelId={typeof model === "string" ? model : undefined} section="chats" />;
}
