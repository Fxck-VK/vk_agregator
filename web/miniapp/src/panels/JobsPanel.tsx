import { useEffect, useState, useCallback } from 'react';
import {
  Button,
  Cell,
  Group,
  Header,
  List,
  PanelHeader,
  Placeholder,
  Spinner,
  Title,
  Text,
  Badge,
} from '@vkontakte/vkui';
import { Icon28AddOutline } from '@vkontakte/icons';
import { api, Job, OPERATION_LABELS, STATUS_LABELS } from '../api';

interface Props {
  onNewJob: () => void;
  onViewJob: (jobId: string) => void;
}

function statusBadge(status: string): 'accent' | 'positive' | 'negative' | 'prominent' | undefined {
  if (status === 'succeeded') return 'positive';
  if (status === 'failed_terminal' || status === 'rejected') return 'negative';
  if (status === 'awaiting_payment') return 'prominent';
  return 'accent';
}

export function JobsPanel({ onNewJob, onViewJob }: Props) {
  const [jobs, setJobs] = useState<Job[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const resp = await api.listJobs(20, 0);
      setJobs(resp.items);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Ошибка загрузки');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  return (
    <>
      <PanelHeader
        after={
          <Button
            mode="primary"
            size="s"
            before={<Icon28AddOutline />}
            onClick={onNewJob}
          >
            Создать
          </Button>
        }
      >
        Мои задачи
      </PanelHeader>

      {loading && (
        <Group>
          <Spinner size="large" style={{ margin: '40px auto', display: 'block' }} />
        </Group>
      )}

      {!loading && error && (
        <Group>
          <Placeholder>
            <Text>{error}</Text>
            <Button onClick={() => void load()} style={{ marginTop: 12 }}>
              Повторить
            </Button>
          </Placeholder>
        </Group>
      )}

      {!loading && !error && jobs.length === 0 && (
        <Group>
          <Placeholder
            header="Задач пока нет"
            action={
              <Button onClick={onNewJob}>Создать первую задачу</Button>
            }
          >
            Нажмите «Создать», чтобы отправить запрос на генерацию текста,
            изображения или видео.
          </Placeholder>
        </Group>
      )}

      {!loading && !error && jobs.length > 0 && (
        <Group header={<Header mode="secondary">Последние задачи</Header>}>
          <List>
            {jobs.map((job) => (
              <Cell
                key={job.id}
                subtitle={STATUS_LABELS[job.status] ?? job.status}
                after={<Badge mode={statusBadge(job.status)}>{OPERATION_LABELS[job.operation] ?? job.operation}</Badge>}
                onClick={() => onViewJob(job.id)}
              >
                <Title level="3" style={{ fontSize: 14 }}>
                  {OPERATION_LABELS[job.operation] ?? job.operation}
                </Title>
              </Cell>
            ))}
          </List>
        </Group>
      )}
    </>
  );
}
