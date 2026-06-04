// src/screens/JobsPanel.tsx
import { useEffect, useState } from "react";
import { Screen, Card, StatusPill, Spinner } from "../ui/ui";
import { listJobs, opLabel, formatTime, type Job } from "../api/client";

export function JobsPanel({ onOpen }: { onOpen: (id: string) => void }) {
  const [jobs, setJobs] = useState<Job[] | null>(null);
  const [err, setErr] = useState<string | null>(null);

  async function load() {
    setErr(null);
    try {
      setJobs(await listJobs());
    } catch (e) {
      setErr(e instanceof Error ? e.message : "Ошибка загрузки");
    }
  }
  useEffect(() => {
    load();
  }, []);

  return (
    <Screen title="Задачи" right={<button className="linkbtn" onClick={load}>Обновить</button>}>
      {err && <p className="error">{err}</p>}
      {!jobs && !err && <div className="center"><Spinner /></div>}
      {jobs && jobs.length === 0 && <p className="muted">Пока нет задач</p>}
      <div className="list">
        {jobs?.map((j) => (
          <Card key={j.id} onClick={() => onOpen(j.id)}>
            <div className="row">
              <span className="row__title">{opLabel(j.operation)}</span>
              <StatusPill status={j.status} />
            </div>
            <div className="row__meta">{formatTime(j.created_at)}</div>
          </Card>
        ))}
      </div>
    </Screen>
  );
}
