// src/screens/BalanceScreen.tsx
import { useEffect, useState } from "react";
import { Screen, Spinner } from "../ui/ui";
import { getBalance } from "../api/client";

export function BalanceScreen() {
  const [bal, setBal] = useState<number | null>(null);
  const [err, setErr] = useState<string | null>(null);

  async function load() {
    setErr(null);
    try {
      setBal(await getBalance());
    } catch (e) {
      setErr(e instanceof Error ? e.message : "Ошибка");
    }
  }
  useEffect(() => {
    load();
  }, []);

  return (
    <Screen title="Баланс">
      {err && <p className="error">{err}</p>}
      <div className="balance">
        <div className="balance__num">
          {bal === null ? <Spinner size={28} /> : bal.toLocaleString("ru-RU")}
        </div>
        <div className="balance__label">кредитов</div>
      </div>
      <button className="linkbtn center-btn" onClick={load}>Обновить</button>
    </Screen>
  );
}
