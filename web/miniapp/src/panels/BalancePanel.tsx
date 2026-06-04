import { useEffect, useState, useCallback } from 'react';
import {
  Button,
  Group,
  Header,
  Placeholder,
  SimpleCell,
  InfoRow,
  Spinner,
  Text,
  Title,
  Div,
} from '@vkontakte/vkui';
import { Icon28WalletOutline } from '@vkontakte/icons';
import { api, Balance } from '../api';

export function BalancePanel() {
  const [balance, setBalance] = useState<Balance | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const b = await api.getBalance();
      setBalance(b);
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

      {!loading && !error && balance && (
        <>
          <Group>
            <Placeholder
              icon={<Icon28WalletOutline width={56} height={56} />}
              title="Мой баланс"
            >
              <Title level="1" style={{ fontSize: 48, lineHeight: 1 }}>
                {balance.balance_credits}
              </Title>
              <Text style={{ marginTop: 4 }}>кредитов</Text>
            </Placeholder>
          </Group>

          <Group header={<Header>Стоимость операций</Header>}>
            <SimpleCell>
              <InfoRow header="💬 Текст (GPT)">1 кредит</InfoRow>
            </SimpleCell>
            <SimpleCell>
              <InfoRow header="🖼️ Изображение">10 кредитов</InfoRow>
            </SimpleCell>
            <SimpleCell>
              <InfoRow header="🎬 Видео">50 кредитов</InfoRow>
            </SimpleCell>
            <Div>
              <Text style={{ fontSize: 12, color: 'var(--vkui--color_text_secondary)' }}>
                Новые пользователи получают 1 000 кредитов бесплатно.
              </Text>
            </Div>
          </Group>
        </>
      )}
    </>
  );
}
