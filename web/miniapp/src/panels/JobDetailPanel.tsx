import { useEffect, useState, useCallback } from 'react';
import {
  Button,
  Cell,
  Div,
  Group,
  Header,
  InfoRow,
  PanelHeader,
  PanelHeaderBack,
  Placeholder,
  SimpleCell,
  Spinner,
  Text,
} from '@vkontakte/vkui';
import { api, Job, OPERATION_LABELS, STATUS_LABELS } from '../api';

interface Props {
  jobId: string | null;
  onBack: () => void;
}

const TERMINAL_STATUSES = new Set([
  'succeeded',
  'failed_terminal',
  'rejected',
  'cancelled',
  'refunded',
]);

export function JobDetailPanel({ jobId, onBack }: Props) {
  const [job, setJob] = useState<Job | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    if (!jobId) return;
    setLoading(true);
    setError(null);
    try {
      const j = await api.getJob(jobId);
      setJob(j);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Ошибка загрузки');
    } finally {
      setLoading(false);
    }
  }, [jobId]);

  useEffect(() => {
    void load();
  }, [load]);

  // Auto-refresh for non-terminal jobs every 3 seconds.
  useEffect(() => {
    if (!job || TERMINAL_STATUSES.has(job.status)) return;
    const timer = setInterval(() => void load(), 3000);
    return () => clearInterval(timer);
  }, [job, load]);

  if (!jobId) {
    return (
      <>
        <PanelHeader before={<PanelHeaderBack onClick={onBack} />}>Задача</PanelHeader>
        <Placeholder>Задача не выбрана</Placeholder>
      </>
    );
  }

  return (
    <>
      <PanelHeader before={<PanelHeaderBack onClick={onBack} />}>
        Задача
      </PanelHeader>

      {loading && !job && (
        <Group>
          <Spinner size="l" style={{ margin: '40px auto', display: 'block' }} />
        </Group>
      )}

      {error && (
        <Group>
          <Placeholder>
            <Text>{error}</Text>
            <Button onClick={() => void load()} style={{ marginTop: 12 }}>
              Повторить
            </Button>
          </Placeholder>
        </Group>
      )}

      {job && (
        <>
          <Group header={<Header>Детали задачи</Header>}>
            <SimpleCell>
              <InfoRow header="Тип">
                {OPERATION_LABELS[job.operation] ?? job.operation}
              </InfoRow>
            </SimpleCell>
            <SimpleCell>
              <InfoRow header="Статус">
                {STATUS_LABELS[job.status] ?? job.status}
                {!TERMINAL_STATUSES.has(job.status) && (
                  <Spinner size="s" style={{ marginLeft: 8, display: 'inline-block' }} />
                )}
              </InfoRow>
            </SimpleCell>
            <SimpleCell>
              <InfoRow header="Стоимость">
                {job.cost_estimate} кредитов
              </InfoRow>
            </SimpleCell>
            <SimpleCell>
              <InfoRow header="Создана">
                {new Date(job.created_at).toLocaleString('ru-RU')}
              </InfoRow>
            </SimpleCell>
            {job.error_code && (
              <SimpleCell>
                <InfoRow header="Код ошибки">{job.error_code}</InfoRow>
              </SimpleCell>
            )}
          </Group>

          {job.output_artifact_ids.length > 0 && (
            <Group header={<Header>Результат</Header>}>
              {job.output_artifact_ids.map((aid) => (
                <Cell key={aid} subtitle={aid}>
                  Артефакт готов
                </Cell>
              ))}
              <Div>
                <Text style={{ fontSize: 12, color: 'var(--vkui--color_text_secondary)' }}>
                  Результат также доставлен в ваш чат ВКонтакте.
                </Text>
              </Div>
            </Group>
          )}
        </>
      )}
    </>
  );
}
