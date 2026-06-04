import { useState } from 'react';
import {
  Button,
  FormItem,
  FormLayout,
  Group,
  PanelHeader,
  PanelHeaderBack,
  SegmentedControl,
  Textarea,
  Snackbar,
} from '@vkontakte/vkui';
import { Icon24ErrorCircle } from '@vkontakte/icons';
import { api } from '../api';

interface Props {
  onBack: () => void;
  onJobCreated: (jobId: string) => void;
}

const OPERATIONS = [
  { label: '💬 Текст', value: 'text_generate' },
  { label: '🖼️ Фото', value: 'image_generate' },
  { label: '🎬 Видео', value: 'video_generate' },
];

const PLACEHOLDERS: Record<string, string> = {
  text_generate: 'Задайте вопрос или введите запрос для GPT...',
  image_generate: 'Опишите изображение, которое хотите сгенерировать...',
  video_generate: 'Опишите видео, которое хотите сгенерировать...',
};

export function NewJobPanel({ onBack, onJobCreated }: Props) {
  const [operation, setOperation] = useState('text_generate');
  const [prompt, setPrompt] = useState('');
  const [loading, setLoading] = useState(false);
  const [snackbar, setSnackbar] = useState<string | null>(null);

  const handleSubmit = async () => {
    if (!prompt.trim()) return;
    setLoading(true);
    try {
      const job = await api.createJob(operation, prompt.trim());
      onJobCreated(job.id);
    } catch (e) {
      setSnackbar(e instanceof Error ? e.message : 'Ошибка создания задачи');
    } finally {
      setLoading(false);
    }
  };

  return (
    <>
      <PanelHeader before={<PanelHeaderBack onClick={onBack} />}>
        Новая задача
      </PanelHeader>

      <Group>
        <FormLayout>
          <FormItem top="Тип генерации">
            <SegmentedControl
              value={operation}
              onChange={(v) => setOperation(v as string)}
              options={OPERATIONS}
            />
          </FormItem>

          <FormItem top="Запрос">
            <Textarea
              placeholder={PLACEHOLDERS[operation]}
              value={prompt}
              onChange={(e) => setPrompt(e.target.value)}
              rows={5}
              disabled={loading}
            />
          </FormItem>

          <FormItem>
            <Button
              size="l"
              stretched
              loading={loading}
              disabled={!prompt.trim() || loading}
              onClick={() => void handleSubmit()}
            >
              Отправить
            </Button>
          </FormItem>
        </FormLayout>
      </Group>

      {snackbar && (
        <Snackbar
          onClose={() => setSnackbar(null)}
          before={<Icon24ErrorCircle fill="var(--vkui--color_icon_negative)" />}
        >
          {snackbar}
        </Snackbar>
      )}
    </>
  );
}
