// src/App.tsx
import { useState } from "react";
import { useBridgeInit } from "./hooks/useBridge";
import { NewJobPanel } from "./screens/NewJobPanel";
import { JobsPanel } from "./screens/JobsPanel";
import { JobDetailScreen } from "./screens/JobDetailScreen";
import { BalanceScreen } from "./screens/BalanceScreen";
import { Spinner } from "./ui/ui";

type TabId = "create" | "jobs" | "balance";

export default function App() {
  const ready = useBridgeInit();
  const [tab, setTab] = useState<TabId>("create");
  const [jobId, setJobId] = useState<string | null>(null);

  if (!ready) {
    return (
      <div className="app app--center">
        <Spinner size={28} />
      </div>
    );
  }

  return (
    <div className="app">
      <main className="app__main">
        {jobId ? (
          <JobDetailScreen jobId={jobId} onBack={() => setJobId(null)} />
        ) : tab === "create" ? (
          <NewJobPanel onCreated={setJobId} />
        ) : tab === "jobs" ? (
          <JobsPanel onOpen={setJobId} />
        ) : (
          <BalanceScreen />
        )}
      </main>

      {!jobId && (
        <nav className="tabbar">
          <TabButton label="Создать" active={tab === "create"} onClick={() => setTab("create")} />
          <TabButton label="Задачи" active={tab === "jobs"} onClick={() => setTab("jobs")} />
          <TabButton label="Баланс" active={tab === "balance"} onClick={() => setTab("balance")} />
        </nav>
      )}
    </div>
  );
}

function TabButton({ label, active, onClick }: { label: string; active: boolean; onClick: () => void }) {
  return (
    <button className={"tab" + (active ? " is-active" : "")} onClick={onClick}>
      {label}
    </button>
  );
}
