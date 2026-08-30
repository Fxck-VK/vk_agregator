# Workspace How Video Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Заменить заглушку секции «Как работает NeiroHub» предоставленным локальным MP4-видео.

**Architecture:** Существующий `VideoPlayer` уже реализует controls, отложенную загрузку и fallback ошибки. Главная страница передаёт ему источник из централизованного `assetPaths`, а валидатор публичных ассетов получает поддержку `.mp4`.

**Tech Stack:** Next.js 16, React 19, TypeScript, CSS Modules, Vitest, Node test runner.

## Global Constraints

- Публичный путь: `/assets/videos/workspace/neirohub-how-it-works.mp4`.
- Автовоспроизведение не использовать.
- Сохранить `preload="none"`, controls, размеры и скругление существующего `VideoPlayer`.
- Не добавлять постер.
- Не изменять `web/platform/next-env.d.ts`.

---

### Task 1: Проверяемый MP4-ассет

**Files:**
- Modify: `web/platform/scripts/validate-assets.mjs`
- Modify: `web/platform/scripts/validate-assets.test.mjs`
- Modify: `web/platform/src/assets/asset-paths.ts`
- Modify: `web/platform/src/assets/asset-paths.test.ts`
- Create: `web/platform/public/assets/videos/workspace/neirohub-how-it-works.mp4`

**Interfaces:**
- Consumes: исходный файл `C:/Users/Lenovo/Desktop/videoplayback.mp4`.
- Produces: `assetPaths.videos.workspace.howItWorks` со значением `/assets/videos/workspace/neirohub-how-it-works.mp4`.

- [ ] **Step 1: Write failing asset tests**

```ts
expect(assetPaths.videos.workspace.howItWorks).toBe(
  "/assets/videos/workspace/neirohub-how-it-works.mp4",
);
```

В Node-тесте передать `public/assets/videos/workspace/example-video.mp4` в `inspectAssetEntries` и ожидать пустой список ошибок.

- [ ] **Step 2: Run tests and verify RED**

Run: `npx vitest run src/assets/asset-paths.test.ts`

Run: `npm run test:assets`

Expected: тест пути падает из-за отсутствия `videos`, тест валидатора — из-за неподдерживаемого `.mp4`.

- [ ] **Step 3: Add the path and validator support**

```ts
videos: {
  workspace: {
    howItWorks: "/assets/videos/workspace/neirohub-how-it-works.mp4",
  },
},
```

Добавить `.mp4` в `allowedExtensions` и `mp4` в регулярное выражение kebab-case. Скопировать предоставленный файл под утверждённым именем без перекодирования.

- [ ] **Step 4: Run asset tests and verify GREEN**

Run: `npx vitest run src/assets/asset-paths.test.ts`

Run: `npm run test:assets`

Expected: оба набора завершаются успешно.

---

### Task 2: Видео на главной странице

**Files:**
- Modify: `web/platform/src/features/workspace/WorkspaceHome/WorkspaceHome.test.tsx`
- Modify: `web/platform/src/features/workspace/WorkspaceLanding/WorkspaceLanding.tsx`
- Create: `web/platform/src/components/media/VideoPlayer/VideoPlayer.styles.test.ts`
- Modify: `web/platform/src/components/media/VideoPlayer/VideoPlayer.module.css`

**Interfaces:**
- Consumes: `assetPaths.videos.workspace.howItWorks` и существующий `VideoPlayer`.
- Produces: секцию с доступным `<video>` и локальным MP4-источником.

- [ ] **Step 1: Update the workspace test to RED**

Проверить, что статическая разметка содержит `<video`, `controls`, `preload="none"`, локальный путь и не содержит «Видео скоро появится».

- [ ] **Step 2: Run the focused test and verify RED**

Run: `npx vitest run src/features/workspace/WorkspaceHome/WorkspaceHome.test.tsx`

Expected: FAIL, потому что `WorkspaceLanding` пока не передаёт `source`.

- [ ] **Step 3: Pass the local source**

```tsx
<VideoPlayer
  source={{ src: assetPaths.videos.workspace.howItWorks, type: "video/mp4" }}
  title="Как работает NeiroHub"
/>
```

Закрепить видео по всем краям существующего контейнера:

```css
.frame {
  position: relative;
}

.video {
  position: absolute;
  inset: 0;
  inline-size: 100%;
  block-size: 100%;
  object-fit: contain;
}
```

- [ ] **Step 4: Run focused and full verification**

Run: `npx vitest run src/features/workspace/WorkspaceHome/WorkspaceHome.test.tsx src/components/media/VideoPlayer/VideoPlayer.test.tsx src/assets/asset-paths.test.ts`

Run: `npm test`

Run: `npm run typecheck`

Run: `npm run lint`

Run: `npm run build`

Expected: все команды завершаются успешно.

- [ ] **Step 5: Verify locally and commit**

Открыть `http://localhost:7158/app`, прокрутить к секции «Как работает NeiroHub» и убедиться, что отображается видеоплеер. Stage только файлы задания; `web/platform/next-env.d.ts` не включать.

---

### Task 3: Кастомная заставка до первого запуска

**Files:**
- Modify: `web/platform/src/components/media/VideoPlayer/VideoPlayer.test.tsx`
- Modify: `web/platform/src/components/media/VideoPlayer/VideoPlayer.styles.test.ts`
- Modify: `web/platform/src/components/media/VideoPlayer/VideoPlayer.tsx`
- Modify: `web/platform/src/components/media/VideoPlayer/VideoPlayer.module.css`
- Modify: `web/platform/src/features/workspace/WorkspaceHome/WorkspaceHome.test.tsx`

**Interfaces:**
- Consumes: существующие `source`, `title`, `preload="none"` и fallback ошибки `VideoPlayer`.
- Produces: доступную кнопку `Воспроизвести: <title>`, скрывающую заставку и запускающую `<video>`.

- [ ] **Step 1: Write failing interaction and style tests**

До клика ожидать кнопку запуска, отсутствие текста «Видео скоро появится» и отсутствие атрибута controls. После клика ожидать вызов `HTMLMediaElement.play()`, исчезновение кнопки и появление controls.

CSS-контракт фиксирует `.posterOverlay` с `position: absolute`, `inset: 0`, градиентным фоном и `.playButton` круглой формы.

- [ ] **Step 2: Run focused tests and verify RED**

Run: `npx vitest run src/components/media/VideoPlayer/VideoPlayer.test.tsx src/components/media/VideoPlayer/VideoPlayer.styles.test.ts src/features/workspace/WorkspaceHome/WorkspaceHome.test.tsx`

Expected: FAIL, потому что сейчас native controls видны сразу, а отдельной кнопки и overlay нет.

- [ ] **Step 3: Implement the overlay and first-play transition**

Использовать `useRef<HTMLVideoElement>` и состояние `hasStarted`. Рендерить `controls={hasStarted}` и до первого запуска показывать:

```tsx
<div className={styles.posterOverlay}>
  <button
    aria-label={`Воспроизвести: ${title}`}
    className={styles.playButton}
    onClick={startPlayback}
    type="button"
  >
    <span aria-hidden="true">▶</span>
  </button>
</div>
```

`startPlayback` переключает состояние и вызывает `videoRef.current?.play()`. Отклонение промиса возвращает заставку; событие `onError` сохраняет существующий fallback.

- [ ] **Step 4: Run focused and full verification**

Run: `npx vitest run src/components/media/VideoPlayer/VideoPlayer.test.tsx src/components/media/VideoPlayer/VideoPlayer.styles.test.ts src/features/workspace/WorkspaceHome/WorkspaceHome.test.tsx`

Run: `npm test`

Run: `npm run typecheck`

Run: `npm run lint`

Run: `npm run build`

Expected: все команды завершаются успешно.

- [ ] **Step 5: Verify locally and commit**

На `http://localhost:7158/app#workspace-how-title` до клика должны быть видны только градиент и центральная кнопка. После клика должны исчезнуть overlay, начаться воспроизведение и появиться стандартные controls. Stage только файлы задания; `web/platform/next-env.d.ts` не включать.
