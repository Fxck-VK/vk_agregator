import { webBrowserFetch } from "@/lib/web-api/browser";
import { parseChatModelList } from "@/lib/web-api/contracts";

export async function loadChatModelCatalog() {
  const response = await webBrowserFetch("/web/v1/chat-models");
  if (response.status !== 200) throw new Error("Unable to load chat models.");
  return parseChatModelList(await response.json());
}
