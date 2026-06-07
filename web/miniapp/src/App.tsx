// src/App.tsx
import { useBridge } from "./hooks/useBridge";
import { ChatScreen } from "./chat/ChatScreen";
import { Spinner } from "./ui/ui";

export default function App() {
  const { ready, user } = useBridge();

  if (!ready) {
    return (
      <div className="splash">
        <Spinner size={26} />
      </div>
    );
  }

  return <ChatScreen user={user ?? { firstName: "Пользователь", name: "Пользователь", avatar: null }} />;
}
