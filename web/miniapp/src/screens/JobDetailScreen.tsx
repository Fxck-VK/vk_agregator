// src/screens/JobDetailScreen.tsx
import { useCallback, useEffect, useState } from "react";
import { Screen, Card, StatusPill, Spinner } from "../ui/ui";
import { getJob, isTerminal, opLabel, formatTime, type Job } from "../api/client";

export function JobDetailScreen({ jobId, onBack }: { jobId: string; onBack: () => void }) {
  const [job, setJob] = useState<Job | null>(null);
  const [err, setErr] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      setJob(await getJob(jobId));
    } catch (e) {
      setErr(e instanceof Error ? e.message : "Ошибка");
    }
  }, [jobId]);

  useEffect(() => {
    load();
  }, [load]);

  // Поллинг, пока статус не финальный.
  useEffect(() => {
    if (!job || isTerminal(job.status)) return;
    const t = setInterval(load, 2000);
    return () => clearInterval(t);
  }, [job, load]);

  return (
    <Screen
      title={job ? opLabel(job.operation) : "Задача"}
      right={<button className="linkbtn" onClick={onBack}>Назад</button>}
    >
      {err && <p className="error">{err}</p>}
      {!job && !err && <div className="center"><Spinner /></div>}
      {job && (
        <>
          <Card>
            <div className="row">
              <span className="muted">Статус</span>
              <StatusPill status={job.status} />
            </div>
            {!isTerminal(job.status) && (
              <div className="row__meta with-spinner">
                <Spinner size={14} /> Обрабатывается…
              </div>
            )}
          </Card>

          {job.prompt && (
            <Card>
              <div className="muted">Запрос</div>
              <div>{job.prompt}</div>
            </Card>
          )}

          <Card>
            <div className="kv"><span className="muted">Создано</span><span>{formatTime(job.created_at)}</span></div>
            <div className="kv"><span className="muted">Оценка</span><span>{job.cost_estimate} кр.</span></div>
            {job.cost_captured > 0 && (
              <div className="kv"><span className="muted">Списано</span><span>{job.cost_captured} кр.</span></div>
            )}
          </Card>

          {job.error_code && (
            <Card><div className="error">{job.error_code}</div></Card>
          )}
        </>
      )}
    </Screen>
  );
}
