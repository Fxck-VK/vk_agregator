import { ConversationHistory } from "@/features/conversations/ConversationHistory/ConversationHistory";
import { loadConversationHistory } from "@/features/conversations/conversation-history-data";

export default async function ConversationPage({
  params,
  searchParams,
}: Readonly<{
  params: Promise<{ conversationId: string }>;
  searchParams: Promise<{ refresh?: string | string[] }>;
}>) {
  const { conversationId } = await params;
  const { refresh } = await searchParams;
  const history = await loadConversationHistory(conversationId);

  return <ConversationHistory history={history} initialRefresh={refresh === "1"} />;
}
