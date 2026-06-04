import { useEffect, useState, useCallback } from 'react';
import {
  Button,
  Group,
  Header,
  List,
  PanelHeader,
  Placeholder,
  SimpleCell,
  Spinner,
  Text,
} from '@vkontakte/vkui';
import { Icon28AddOutline, Icon24ChevronRight } from '@vkontakte/icons';
import { api, Job, OPERATION_LABELS, STATUS_LABELS } from '../api';

interface Props {
  onNewJob: () => void;
  onViewJob: (jobId: string) => void;
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
          <Spinner size="l" style={{ margin: '40px auto', display: 'block' }} />
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
            title="Задач пока нет"
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
        <Group header={<Header>Последние задачи</Header>}>
          <List>
            {jobs.map((job) => (
              <SimpleCell
                key={job.id}
                subtitle={STATUS_LABELS[job.status] ?? job.status}
                after={<Icon24ChevronRight />}
                onClick={() => onViewJob(job.id)}
              >
                {OPERATION_LABELS[job.operation] ?? job.operation}
              </SimpleCell>
            ))}
          </List>
        </Group>
      )}
    </>
  );
}
