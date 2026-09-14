# Матрица проверки существующих моделей — 2026-09-14

Снимок `go run ./scripts/models/check -bindings`: 43 записи, без изменения
замороженных отпечатков. Публичная офлайн-выдача содержит 36 web-пригодных записей
при синтетически включённых маршрутах. Это не доступность DEV/PROD.

«Варианты» — число опубликованных комбинаций качества/пропорций или
разрешения/длительности/пропорций. Тест `TestEveryPreviewOptionResolvesWithoutCallingProviders`
проверяет их через существующие resolver без обращения к провайдерам. Он не
подменяет проверки адаптера, реального результата и lifecycle всей цепочки.

| Тип | Public ID | Офлайн web-каталог | Варианты | Допуск | Live-output |
| --- | --- | --- | ---: | --- | --- |
| text | `chatgpt` | есть | 1 | legacy-unverified | not_run |
| text | `gpt_5_5` | есть | 1 | legacy-unverified | not_run |
| text | `claude_opus_4_7` | есть | 1 | legacy-unverified | not_run |
| text | `gemini_3_1_pro` | есть | 1 | legacy-unverified | not_run |
| text | `claude_opus_4_8` | есть | 1 | legacy-unverified | not_run |
| text | `gpt_5_6_terra` | есть | 1 | legacy-unverified | not_run |
| text | `gpt_6_astra` | есть | 1 | legacy-unverified | not_run |
| text | `claude_opus_5` | есть | 1 | legacy-unverified | not_run |
| text | `gemini_3_7_flash` | есть | 1 | legacy-unverified | not_run |
| text | `claude_fable_5_1` | есть | 1 | legacy-unverified | not_run |
| text | `claude_fable_5` | есть | 1 | legacy-unverified | not_run |
| text | `gemini_3_6_flash` | есть | 1 | legacy-unverified | not_run |
| image | `gpt_image_2_5_flare` | есть | 225 | legacy-unverified | not_run |
| image | `gpt_image_2_5_sunburst` | есть | 225 | legacy-unverified | not_run |
| image | `seedream_5_0_lite` | есть | 24 | legacy-unverified | not_run |
| image | `seedream_5_0_pro` | есть | 30 | legacy-unverified | not_run |
| image | `midjourney_v7` | есть | 30 | legacy-unverified | not_run |
| image | `flux_2_pro` | есть | 36 | legacy-unverified | not_run |
| image | `nano_banana_2` | есть | 30 | legacy-unverified | not_run |
| image | `nano_banana_pro` | есть | 30 | legacy-unverified | not_run |
| image | `gpt_image_2` | есть | 30 | legacy-unverified | not_run |
| image | `qwen_image_3` | есть | 20 | legacy-unverified | not_run |
| image | `grok_image_1_5` | есть | 5 | legacy-unverified | not_run |
| image | `grok_image_2_0` | есть | 7 | legacy-unverified | not_run |
| image | `seedream_4_5` | есть | 20 | legacy-unverified | not_run |
| image | `mock_image` | только loadtest | - | legacy-unverified | not_run |
| video | `video_kling_v3` | есть | 117 | legacy-unverified | not_run |
| video | `video_kling_2_6_motion_control` | нет: нужен иной входной путь | - | legacy-unverified | not_run |
| video | `video_veo_3_1_fast` | есть | 6 | legacy-unverified | not_run |
| video | `video_veo_3_1_quality` | есть | 6 | legacy-unverified | not_run |
| video | `video_veo_3_1_lite` | есть | 6 | legacy-unverified | not_run |
| video | `video_kling_3_0_turbo` | есть | 78 | legacy-unverified | not_run |
| video | `video_minimax_h3` | есть | 144 | legacy-unverified | not_run |
| video | `video_gemini_omni_1_1_flash` | нет: нужен иной входной путь | - | legacy-unverified | not_run |
| video | `video_gemini_omni_1_1_flash_ext` | есть | 32 | legacy-unverified | not_run |
| video | `video_seedance_2_5` | есть | 72 | legacy-unverified | not_run |
| video | `video_hailuo_2_3_fast` | нет: нужен иной входной путь | - | legacy-unverified | not_run |
| video | `video_hailuo_2_3_standard` | нет: нужен иной входной путь | - | legacy-unverified | not_run |
| video | `video_kling_o3_standard` | есть | 12 | legacy-unverified | not_run |
| video | `video_runway_gen4_turbo` | нет: нужен иной входной путь | - | legacy-unverified | not_run |
| video | `video_seedance_2_0_fast` | есть | 6 | legacy-unverified | not_run |
| video | `video_runway_gen4_5` | есть | 24 | legacy-unverified | not_run |
| video | `video_mock_text_to_video` | только loadtest | - | legacy-unverified | not_run |

## Оставшиеся проверки для каждой строки

1. Уточнить актуальную официальную документацию именно подключённого провайдера,
   версию и ограничения. Сохранить ссылки/дату и сопоставить с привязкой реестра.
2. Заполнить typed contract: входные форматы/размеры/количество, нативная обработка
   или извлечение текста; варианты выхода; значения по умолчанию; стоимость.
   Неизвестные сведения оставить unknown с enabled=false.
3. Выполнить adapter, negative, boundaries, pricing, job-lifecycle и catalog
   проверки для точной версии контракта. Матрица выше подтверждает только
   офлайн-проекцию доступных вариантов, а не все эти сценарии допуска.
4. После разрешения DEV-контура и предела расходов выполнить live-output:
   по одному базовому результату для каждой реальной операции; выбранные
   минимальные/максимальные параметры; по одному различимому синтетическому
   примеру на каждый включаемый входной формат. Mock-записи проверять только
   в loadtest и никогда не выдавать за подтверждение реального провайдера.
5. Проверить результат: текст и фактическое использование вложения; декодирование
   и размеры изображения; длительность, размеры, FPS и аудиодорожку видео.
   Для референсов проверить их использование и обязательность начального/конечного
   кадра. Сохранить честную оценку и непокрытые варианты.
6. Привязать отчёты и их хеши к digest контракта. Только после успешного допуска
   удалить соответствующее legacy-исключение и повторить общий registry check.

Live-вызовы в этой работе не запускались; разрешение провайдера/DEV-контура и
денежный предел ещё не заданы. Базовый план — проверка всех реальных записей
последовательно с остановкой при исчерпании согласованного бюджета или первом
несоответствии безопасности/списания. Число дополнительных вызовов определяется
после сверки документации и реально включаемых форматов. Точная денежная оценка
требует актуальных тарифов поставщика: внутренние кредиты сайта не являются ею.

Правила и шаблоны: [MODEL_ONBOARDING.md](../MODEL_ONBOARDING.md).
Единая выдача: [MODEL_CATALOG.md](../MODEL_CATALOG.md).