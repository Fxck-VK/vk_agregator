import { ConversationHistory } from "@/features/conversations/ConversationHistory/ConversationHistory";
import { loadConversationHistory } from "@/features/conversations/conversation-history-data";

export default async function ConversationPage({
  params,
}: Readonly<{
  params: Promise<{ conversationId: string }>;
}>) {
  const { conversationId } = await params;
  const history = await loadConversationHistory(conversationId);

  return <ConversationHistory history={history} />;
}
