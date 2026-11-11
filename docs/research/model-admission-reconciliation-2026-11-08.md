# Сверка проверок моделей и PROD-допуска — 8 ноября 2026

Статус: аудит кода и исторических результатов. Допуски, runtime и цены не изменены.

Источник: предоставленный `HANDOFF_CURRENT (2).md` от 21 октября 2026; SHA-256 `251fb12d9ffd80f81db219bafb9101ac9e32c97896691ee2541372a8c2863ac5`.
Текущий HEAD: `c58f310dab3ceef54ff4c16d072546434b5fbd38`. Незакоммиченные изменения других задач сохранены.

## Основные результаты

- В источнике поимённо описаны 59 моделей: 35 текстовых, 18 изображений и 6 видео (включая непринятый Vidu Q3 Pro).
- Базовых успехов в источнике: 46. Из них 36 сейчас исключены из локального PROD: 29 текстовых, 6 изображений, PixVerse v6.
- Локальный фильтр PROD пропускает 36 моделей; у 23 из них нет индивидуального результата в этом документе. Это пробел данного источника, не утверждение об отсутствии любых проверок.
- Три модели с частичными результатами уже входят в PROD: Nano Banana 2, Grok Imagine 1.5 и Seedream 4.5.
- У 29 успешных текстовых кандидатов и 7 успешных media-кандидатов остаются draft-контракты / pending-verification. Заготовки проверок не заполнены историческим результатом.
- Пять точных недоступных текстовых маршрутов остаются закрыты; их исторический отказ не потерян.
- Коммиты `d2c56ef6` и `73426de6` уже являются предками текущего HEAD; повторный merge/cherry-pick не требуется.

## Матрица исторических моделей

| Тип | Модель / ID | Результат 21.10 | Локальный PROD | Каталог / контракт |
|---|---|---|---|---|
| text | NeiroHub Chat / `chatgpt` | Базовый проход | да | `legacy-unverified` / `legacy-unverified` |
| text | GPT-5 / `gpt_5` | Базовый проход | нет | `pending-verification` / `draft` |
| text | GPT-5.1 / `gpt_5_1` | Базовый проход | нет | `pending-verification` / `draft` |
| text | GPT-5 Chat Latest / `gpt_5_chat_latest` | Отказ маршрута | нет | `pending-verification` / `draft` |
| text | GPT-5 Mini / `gpt_5_mini` | Базовый проход | нет | `pending-verification` / `draft` |
| text | Claude Opus 4.6 / `claude_opus_4_6` | Базовый проход | нет | `pending-verification` / `draft` |
| text | Claude Sonnet 4.6 / `claude_sonnet_4_6` | Базовый проход | нет | `pending-verification` / `draft` |
| text | Claude Opus 4.5 / `claude_opus_4_5_20251101` | Базовый проход | нет | `pending-verification` / `draft` |
| text | Gemini 3.5 Flash / `gemini_3_5_flash` | Базовый проход | нет | `pending-verification` / `draft` |
| text | Gemini 3.1 Pro Preview / `gemini_3_1_pro_preview` | Базовый проход | нет | `pending-verification` / `draft` |
| text | Gemini 3 Pro Preview / `gemini_3_pro_preview` | Базовый проход | нет | `pending-verification` / `draft` |
| text | Gemini 3 Pro Preview Thinking / `gemini_3_pro_preview_thinking` | Базовый проход | нет | `pending-verification` / `draft` |
| text | Gemini 3 Flash Preview / `gemini_3_flash_preview` | Базовый проход | нет | `pending-verification` / `draft` |
| text | Gemini 2.5 Pro / `gemini_2_5_pro` | Базовый проход | нет | `pending-verification` / `draft` |
| text | Gemini 2.5 Flash / `gemini_2_5_flash` | Базовый проход | нет | `pending-verification` / `draft` |
| text | Gemini 2.5 Flash Lite / `gemini_2_5_flash_lite` | Базовый проход | нет | `pending-verification` / `draft` |
| text | DeepSeek V4 Pro / `deepseek_v4_pro` | Базовый проход | нет | `pending-verification` / `draft` |
| text | DeepSeek V4 Flash / `deepseek_v4_flash` | Базовый проход | нет | `pending-verification` / `draft` |
| text | DeepSeek V3.2 / `deepseek_v3_2` | Отказ маршрута | нет | `pending-verification` / `draft` |
| text | DeepSeek V3.2 Exp / `deepseek_v3_2_exp` | Отказ маршрута | нет | `pending-verification` / `draft` |
| text | DeepSeek R1 / `deepseek_r1_250528` | Отказ маршрута | нет | `pending-verification` / `draft` |
| text | DeepSeek V3 / `deepseek_v3_0324` | Отказ маршрута | нет | `pending-verification` / `draft` |
| text | GPT-6 Sol / `gpt_6_sol` | Базовый проход | нет | `pending-verification` / `draft` |
| text | GPT-6 Luna / `gpt_6_luna` | Базовый проход | нет | `pending-verification` / `draft` |
| text | GPT-5.4 / `gpt_5_4` | Базовый проход | нет | `pending-verification` / `draft` |
| text | GPT-5.3 Codex / `gpt_5_3_codex` | Базовый проход | нет | `pending-verification` / `draft` |
| text | GPT-5.2 / `gpt_5_2` | Базовый проход | нет | `pending-verification` / `draft` |
| text | Claude Opus 5.5 / `claude_opus_5_5` | Базовый проход | нет | `pending-verification` / `draft` |
| text | Claude Haiku 4.5 / `claude_haiku_4_5_20251001` | Базовый проход | нет | `pending-verification` / `draft` |
| text | Kimi K3 / `kimi_k3` | Базовый проход | нет | `pending-verification` / `draft` |
| text | Qwen 3.8 Max / `qwen_3_8_max` | Базовый проход | нет | `pending-verification` / `draft` |
| text | Qwen 3.7 Flash / `qwen_3_7_flash` | Базовый проход | нет | `pending-verification` / `draft` |
| text | Grok 4.5 / `grok_4_5` | Базовый проход | нет | `pending-verification` / `draft` |
| text | Grok 4.6 / `grok_4_6` | Базовый проход | нет | `pending-verification` / `draft` |
| text | Grok 4.7 / `grok_4_7` | Базовый проход | нет | `pending-verification` / `draft` |
| image | GPT Image 2.5 Flare / `gpt_image_2_5_flare` | Базовый проход | да | `legacy-unverified` / `legacy-unverified` |
| image | GPT Image 2.5 Sunburst / `gpt_image_2_5_sunburst` | Базовый проход | да | `legacy-unverified` / `legacy-unverified` |
| image | Seedream 5.0 Lite / `seedream_5_0_lite` | Базовый проход | да | `legacy-unverified` / `legacy-unverified` |
| image | Seedream 5.0 Pro / `seedream_5_0_pro` | Базовый проход | да | `legacy-unverified` / `legacy-unverified` |
| image | Nano Banana 2 / `nano_banana_2` | Частичный результат | да | `legacy-unverified` / `legacy-unverified` |
| image | Nano Banana Pro / `nano_banana_pro` | Базовый проход | да | `legacy-unverified` / `legacy-unverified` |
| image | GPT Image 2 / `gpt_image_2` | Базовый проход | да | `legacy-unverified` / `legacy-unverified` |
| image | Qwen Image 3.0 / `qwen_image_3` | Базовый проход | да | `legacy-unverified` / `legacy-unverified` |
| image | Grok Imagine 1.5 / `grok_image_1_5` | Частичный результат | да | `legacy-unverified` / `legacy-unverified` |
| image | Grok Imagine 2.0 / `grok_image_2_0` | Базовый проход | да | `legacy-unverified` / `legacy-unverified` |
| image | Seedream 4.5 / `seedream_4_5` | Частичный результат | да | `legacy-unverified` / `legacy-unverified` |
| image | Imagen 4.0 / `imagen_4_0` | Сбой | нет | `pending-verification` / `draft` |
| image | Nano Banana / `nano_banana` | Базовый проход | нет | `pending-verification` / `draft` |
| image | Seedream 5.0 Flash / `seedream_5_0_flash` | Базовый проход | нет | `pending-verification` / `draft` |
| image | Z-Image Turbo / `z_image_turbo` | Базовый проход | нет | `pending-verification` / `draft` |
| image | FLUX.2 Max / `flux_2_max` | Базовый проход | нет | `pending-verification` / `draft` |
| image | FLUX.2 Flex / `flux_2_flex` | Базовый проход | нет | `pending-verification` / `draft` |
| image | Qwen Image 3.0 Pro / `qwen_image_3_pro` | Базовый проход | нет | `pending-verification` / `draft` |
| video | PixVerse v6 / `pixverse_v6` | Базовый проход | нет | `pending-verification` / `draft` |
| video | Seedance 2.0 Mini / `seedance_2_0_mini` | Сбой | нет | `pending-verification` / `draft` |
| video | Wan 3.0 Video / `wan_3_0` | Сбой | нет | `pending-verification` / `draft` |
| video | Vidu Q3 Turbo / `vidu_q3_turbo` | Сбой | нет | `pending-verification` / `draft` |
| video | Veo 3.1 Lite / `video_veo_3_1_lite` | Базовый проход | да | `legacy-unverified` / `legacy-unverified` |
| video | Vidu Q3 Pro / `vidu_q3_pro` | Job не принят | нет | `pending-verification` / `draft` |

## Уже в PROD, но без индивидуальной проверки в приложенном документе

| Тип | Модель / ID |
|---|---|
| image | Midjourney V7 / `midjourney_v7` |
| image | FLUX.2 Pro / `flux_2_pro` |
| text | GPT-5.5 / `gpt_5_5` |
| text | Claude Opus 4.7 / `claude_opus_4_7` |
| text | Gemini 3.1 Pro / `gemini_3_1_pro` |
| text | Claude Opus 4.8 / `claude_opus_4_8` |
| text | GPT 5.6 Terra / `gpt_5_6_terra` |
| text | GPT 6 Astra / `gpt_6_astra` |
| text | Claude Opus 5 / `claude_opus_5` |
| text | Gemini 3.7 Flash / `gemini_3_7_flash` |
| text | Claude Fable 5.1 / `claude_fable_5_1` |
| text | Claude Fable 5 / `claude_fable_5` |
| text | Gemini 3.6 Flash / `gemini_3_6_flash` |
| video | Kling V3 / `video_kling_v3` |
| video | Veo 3.1 Fast / `video_veo_3_1_fast` |
| video | Veo 3.1 Quality / `video_veo_3_1_quality` |
| video | Kling 3.0 Turbo / `video_kling_3_0_turbo` |
| video | MiniMax H3 / `video_minimax_h3` |
| video | Gemini Omni 1.1 Flash EXT / `video_gemini_omni_1_1_flash_ext` |
| video | Seedance 2.5 / `video_seedance_2_5` |
| video | Kling O3 Standard / `video_kling_o3_standard` |
| video | Seedance 2.0 Fast / `video_seedance_2_0_fast` |
| video | Runway Gen-4.5 / `video_runway_gen4_5` |

## Что нужно согласовать с допуском

1. Сохранить 30 успешных текстовых прогонов как историческое свидетельство `reply` с текстовым входом. Для 29 кандидатов сверить существующие отрицательные/граничные/ценовые/Job-проверки и оформить привязанный контракт допуска; не подменять этот шаг удалением фильтра или расширением legacy.json.
2. В отдельную очередь допуска поставить шесть успешных image-кандидатов: Nano Banana, Seedream 5.0 Flash, Z-Image Turbo, FLUX.2 Max, FLUX.2 Flex, Qwen Image 3.0 Pro. Источник не сообщает выбранные quality-коды и точный runtime SHA для media — переносить только зафиксированный объём результата.
3. Для PixVerse v6 зафиксирован один успешный режим 360p / 1 секунда / 16:9. Для Veo 3.1 Lite — 720p / 8 секунд / 16:9 и воспроизведение; FPS/audio/полный ffprobe в источнике не закончены. Не переносить этот результат на остальные режимы.
4. Nano Banana 2, Grok Imagine 1.5 и Seedream 4.5 оставить в списке замечаний по preview/соотношению сторон. Imagen 4.0, Seedance 2.0 Mini, Wan 3.0 Video и Vidu Q3 Turbo не помечать прошедшими. Vidu Q3 Pro не получил принятого Job.
5. Для ещё 24 неназванных видео найти приложение `media-models-qa.json` из передачи; не назначать им текущие public ID по предположению.

## Сверка исходников

Точные определения кандидатов и перечисленные ниже адаптеры/валидаторы не изменены относительно коммита фикса из передачи. Это проверка Git-исходников; сам документ не подтверждает SHA runtime во время media-запусков.

- `internal/service/providermodels/text_candidates.go` — без изменений относительно `d2c56ef6`.
- `internal/service/providermodels/media_candidates.go` — без изменений относительно `d2c56ef6`.
- `internal/service/providermodels/next_media_candidates.go` — без изменений относительно `d2c56ef6`.
- `internal/service/providermodels/catalog_expansion.go` — без изменений относительно `d2c56ef6`.
- `internal/service/providermodels/image_expansion.go` — без изменений относительно `d2c56ef6`.
- `internal/service/providermodels/video_expansion.go` — без изменений относительно `d2c56ef6`.
- `internal/adapter/provider/poyo/poyo.go` — без изменений относительно `d2c56ef6`.
- `internal/worker/media_contracts.go` — без изменений относительно `d2c56ef6`.
- `internal/service/imagepreview/service.go` — без изменений относительно `d2c56ef6`.
- `web/platform/src/features/files/image-preview.server.ts` — без изменений относительно `d2c56ef6`.

Описания возможностей изображений (`capabilities_image.go`) и их тесты менялись после передачи; это отдельно от неизменённых маршрутов кандидатов. Файлы legacy migration — совместимость прежних определений, не отчёты о реальном результате.

В `onboarding/approved` нет готовых JSON-контрактов (только README). Поэтому ни число старых моделей в PROD, ни status legacy-unverified не означают, что именно этот отчёт подтвердил их полный допуск.

## Ограничения и выполненные проверки

- Общий offline model checker прошёл: 108 моделей, 4039 ценовых вариантов; ожидаемые legacy-unverified сохранены. Он не вызывает провайдеров и не подтверждает новый live-результат.
- Проверены имена, уникальность и суммы строк: 35 text + 18 image + 6 named video = 59; 46 базовых успехов; 36 исторически успешных моделей вне PROD.
- По исходникам строгая проверка video aspect ratio и разрешённые MIME preview по-прежнему содержат участки из исторических замечаний. Новые live-причины ошибок не подтверждались.
- Старые суммы в колонке исторических деталей JSON сохранены как цитируемые факты старого отчёта; применять их к действующей экономике нельзя.
- Ограничения, неизвестные данные и исходные SHA-256 перечислены в JSON. CSV содержит все поимённые результаты и текущие PROD-модели без записи в этом источнике.
- Платных прогонов, provider API, платежей и изменений допуска нет. Коммиты/push/deploy/merge не выполнялись.

Файлы: [JSON](model-admission-reconciliation-2026-11-08.json), [CSV](model-admission-reconciliation-2026-11-08.csv).
