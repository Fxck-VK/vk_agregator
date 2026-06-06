# Merge Runbook: `serega` + Mini App branch

Этот файл - пошаговая инструкция для агента, который будет сливать нашу VK bot ветку с веткой Mini App.

Предполагаемая ветка друга: `origin/fastlife_dev`.
Если актуальная ветка друга называется иначе, заменить ее в командах.

## 1. Preflight

```powershell
git fetch origin --prune
git status --short --branch
git branch --all --verbose --no-abbrev
```

Ожидания:

- рабочее дерево чистое перед merge;
- `.env`, `*.log`, `*.err` не трогать;
- текущие изменения `serega` должны быть предварительно закоммичены и запушены;
- не использовать `git add .`.

Если есть незакоммиченные изменения в коде или docs, сначала остановиться и решить, что с ними делать.

## 2. Рекомендуемая стратегия

Создать отдельную integration branch от актуального `main`, затем влить обе ветки.

```powershell
git checkout main
git pull origin main
git checkout -b feature/merge-serega-fastlife
git merge --no-ff origin/serega -m "merge: bring VK bot branch"
git merge --no-ff origin/fastlife_dev -m "merge: bring Mini App branch"
```

Если вторая ветка друга другая:

```powershell
git merge --no-ff origin/<friend-branch> -m "merge: bring Mini App branch"
```

Почему так:

- `main` остается чистой базой;
- обе feature-ветки сохраняются;
- conflict resolution изолирован в отдельной ветке;
- можно прогнать тесты до push в `main`.

## 3. Если будет конфликт

Сначала посмотреть список:

```powershell
git status --short
git diff --name-only --diff-filter=U
```

Ожидаемые конфликты по прогнозу:

```text
TASKS.md
internal/worker/worker_test.go
```

Но из-за новых незакоммиченных/новых коммитов могут появиться дополнительные конфликты.

## 4. Правила разрешения конфликтов

### `cmd/api/main.go`

Нужно сохранить одновременно:

- VK webhook wiring;
- VK real/mock delivery control client;
- VK profile client;
- Redis dialog state;
- anti-spam service;
- referral service;
- Mini App BFF wiring;
- Mini App auth/session/rate limits;
- health/admin/metrics.

Нельзя решать конфликт выбором только одной стороны.

### `internal/platform/config/config.go`

Сохранить union всех env/config fields:

- OpenAI;
- DeepInfra;
- provider chain/router;
- VK delivery/menu/dialog/referral;
- text context;
- anti-spam;
- Mini App;
- DB/Redis/MinIO;
- moderation/scanning.

После merge обязательно проверить `config_test.go`.

### `internal/worker/generation.go`

Сохранить:

- Mini App job flow и model_id/model selection друга;
- нашу text context сборку;
- provider dispatch/router;
- artifact creation;
- moderation;
- delivery/capture.

VK handler и Mini App handler не должны напрямую вызывать provider.

### `internal/worker/worker_test.go`

Ожидаемый конфликт.

Решать через объединение тестов:

- оставить тесты Mini App async/job behavior друга;
- оставить тесты VK dialog context, anti-spam, provider delivery нашей ветки;
- не удалять test helpers, если они нужны только одной стороне;
- после ручного merge прогнать targeted worker tests.

### `TASKS.md`

Ожидаемый конфликт.

Решать через объединение пунктов backlog/progress:

- сохранить Mini App progress друга;
- сохранить VK bot menu/referral/context/anti-spam progress нашей ветки;
- не удалять выполненные пункты только потому, что они находятся в разных секциях;
- если backlog устарел, пометить follow-up явно, а не стирать.

### `internal/service/billingservice/service.go`

Сохранить ledger semantics:

- grants/referral rewards через ledger entries;
- cost estimate/charging изменения Mini App, если есть;
- никаких прямых balance mutations без ledger.

### `web/miniapp/**`

Если конфликт внутри Mini App frontend, по умолчанию сохранять сторону друга.
Наша ветка не должна менять UX Mini App, кроме backend-compatible будущих hooks.

### Docs

Docs конфликтовать будут часто.
Правило: не удалять разделы, а объединять.

Сохранить упоминания:

- VK bot menu UX;
- current visible buttons;
- referral system;
- dialog context;
- anti-spam;
- Mini App progress;
- known follow-ups.

## 5. Явные checks после merge

Backend targeted:

```powershell
go test ./internal/adapter/inbound/vk
go test ./internal/adapter/inbound/miniapp
go test ./internal/service/referralservice ./internal/service/dialogcontext ./internal/service/antispam
go test ./internal/worker
go test ./internal/platform/config
```

Full backend:

```powershell
go test ./...
go vet ./...
```

Frontend Mini App, если package scripts есть:

```powershell
cd web/miniapp
npm install
npm run build
cd ../..
```

Smoke после merge:

- VK callback confirmation;
- VK `/start`;
- `Показать меню`;
- `Спросить у НейроХаб`;
- ordinary text outside mode;
- `Мой аккаунт`;
- Mini App open/load;
- Mini App job submit.

## 6. Перед push

```powershell
git status --short --branch
git diff --check
```

Проверить вручную:

- `.env` не staged и не изменен;
- нет secret values в diff;
- нет force push;
- feature branches не удаляются;
- merge commit message понятно описывает обе стороны.

## 7. Что передать человеку в отчете

В финальном отчете указать:

- итоговый branch name;
- merge base;
- какие ветки слиты;
- список конфликтов;
- как решены конфликты;
- какие checks прошли;
- какие checks не запускались и почему;
- подтверждение, что `.env` не тронут;
- ссылку на PR или push SHA, если был push.
