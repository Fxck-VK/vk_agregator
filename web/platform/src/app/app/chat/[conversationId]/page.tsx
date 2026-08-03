import { ConversationHistoryLoader } from "@/features/conversations/ConversationHistoryLoader/ConversationHistoryLoader";

export default async function ConversationPage({
  params,
  searchParams,
}: Readonly<{
  params: Promise<{ conversationId: string }>;
  searchParams: Promise<{ refresh?: string | string[] }>;
}>) {
  const { conversationId } = await params;
  const { refresh } = await searchParams;

  return <ConversationHistoryLoader key={conversationId} conversationId={conversationId} initialRefresh={refresh === "1"} />;
}
