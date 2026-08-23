# План реализации количества изображений

1. Добавить тест компонента счётчика: границы 1–4, disabled и callbacks.
2. Добавить frontend-тест сценария: увеличение количества меняет цену и `output_count` в API.
3. Добавить backend-тесты resolver: значение по умолчанию, умножение цены, превышение лимита.
4. Провести `max_output_count` через provider registry, model catalog, product catalog и web API contract.
5. Провести `output_count` через handler, idempotency replay, worker и provider request.
6. Передавать количество в APIMart и PoYo через `n`.
7. Запустить целевые frontend/backend тесты, затем сборку frontend и полный Go test.
