import { ConversationHistoryLoader } from "@/features/conversations/ConversationHistoryLoader/ConversationHistoryLoader";
import { PendingConversationBootstrap } from "@/features/conversations/PendingConversationBootstrap/PendingConversationBootstrap";

export default async function ConversationPage({
  params,
  searchParams,
}: Readonly<{
  params: Promise<{ conversationId: string }>;
  searchParams: Promise<{ pending?: string | string[]; refresh?: string | string[] }>;
}>) {
  const { conversationId } = await params;
  const { pending, refresh } = await searchParams;

  if (pending === "1") {
    return <PendingConversationBootstrap key={conversationId} conversationKey={conversationId} />;
  }

  return <ConversationHistoryLoader key={conversationId} conversationId={conversationId} initialRefresh={refresh === "1"} />;
}
