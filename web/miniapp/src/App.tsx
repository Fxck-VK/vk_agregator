// src/App.tsx
import { useEffect } from "react";
import { useBridge } from "./hooks/useBridge";
import { ChatScreen } from "./chat/ChatScreen";
import { Spinner } from "./ui/ui";
import { acceptReferral, referralCodeFromLocation } from "./api/client";

export default function App() {
  const { ready, user } = useBridge();

  useEffect(() => {
    if (!ready) return;
    const code = referralCodeFromLocation();
    if (!code) return;
    void acceptReferral(code).catch(() => {
      /* referral acceptance is a safe control-path no-op on failure */
    });
  }, [ready]);

  if (!ready) {
    return (
      <div className="splash">
        <Spinner size={26} />
      </div>
    );
  }

  return <ChatScreen user={user ?? { firstName: "Пользователь", name: "Пользователь", avatar: null }} />;
}
