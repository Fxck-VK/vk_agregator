# Silent Session Restoration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the visible «Восстанавливаем сессию» screen with a safe workspace-shaped shell, delayed top progress line, and retryable transient-error state.

**Architecture:** Keep the existing server-side session decision in `app/app/layout.tsx`. The client-only `SessionRefresh` remains the sole owner of the refresh request and delegates presentation to two independent components: `SessionProgressBar` and `SessionRestorationShell`. No authenticated snapshot is cached or rendered before the server confirms the refreshed session.

**Tech Stack:** Next.js 16 App Router, React 19, TypeScript 5.9, CSS Modules, Vitest 4, Testing Library.

## Global Constraints

- Do not change the backend contract `/web/v1/auth/refresh` or cookie format.
- Do not render profile, email, balance, conversations, or current chat content before successful refresh.
- Delay the progress line by exactly 150 ms and time out one attempt after exactly 8 seconds.
- Treat `400`, `401`, and `403` as invalid-session responses that navigate to `/login`.
- Treat network errors, timeout, `408`, `429`, `5xx`, and any other non-success response as retryable without logging the user out.
- One mount makes one refresh request; each explicit retry makes exactly one additional request.
- Keep authenticated, unauthenticated, and unavailable layout behavior unchanged.
- Use independent component folders with one TSX file, one CSS Module, and focused tests.

---

## File Structure

- Create `web/platform/src/features/session/SessionProgressBar/SessionProgressBar.tsx`: accessible delayed-progress presentation.
- Create `web/platform/src/features/session/SessionProgressBar/SessionProgressBar.module.css`: fixed 3 px animation and reduced-motion fallback.
- Create `web/platform/src/features/session/SessionProgressBar/SessionProgressBar.test.tsx`: visible/hidden accessibility contract.
- Create `web/platform/src/features/session/SessionProgressBar/SessionProgressBar.styles.test.ts`: non-layout animation contract.
- Create `web/platform/src/features/session/SessionRestorationShell/SessionRestorationShell.tsx`: neutral shell and retry state.
- Create `web/platform/src/features/session/SessionRestorationShell/SessionRestorationShell.module.css`: stable desktop/mobile skeleton geometry.
- Create `web/platform/src/features/session/SessionRestorationShell/SessionRestorationShell.test.tsx`: privacy and retry interaction contract.
- Modify `web/platform/src/features/session/SessionRefresh/SessionRefresh.tsx`: refresh state machine, delay, timeout, and retry.
- Modify `web/platform/src/features/session/SessionRefresh/SessionRefresh.test.tsx`: request and timer behavior.
- Delete `web/platform/src/features/session/SessionRefresh/SessionRefresh.module.css`: the old visible card is no longer rendered.
- Modify `web/platform/src/app/app/layout.test.tsx`: assert the safe restoration shell instead of visible copy.
- Modify `web/platform/src/i18n/ru.ts`: add accessible progress, retryable error, and retry labels.

---

### Task 1: Accessible top progress line

**Files:**
- Create: `web/platform/src/features/session/SessionProgressBar/SessionProgressBar.tsx`
- Create: `web/platform/src/features/session/SessionProgressBar/SessionProgressBar.module.css`
- Test: `web/platform/src/features/session/SessionProgressBar/SessionProgressBar.test.tsx`
- Test: `web/platform/src/features/session/SessionProgressBar/SessionProgressBar.styles.test.ts`

**Interfaces:**
- Consumes: `visible: boolean`, `label: string`.
- Produces: `SessionProgressBar({ visible, label }: SessionProgressBarProps): JSX.Element | null`.

- [ ] **Step 1: Write failing component and style-contract tests**

```tsx
it("does not expose a progressbar before the delayed state", () => {
  render(<SessionProgressBar label="Восстановление сессии" visible={false} />);
  expect(screen.queryByRole("progressbar")).not.toBeInTheDocument();
});

it("exposes an accessible progressbar when visible", () => {
  render(<SessionProgressBar label="Восстановление сессии" visible />);
  expect(screen.getByRole("progressbar", { name: "Восстановление сессии" })).toBeInTheDocument();
});
```

The style test must read the CSS file and require `position: fixed`, `block-size: 0.1875rem`, a transform-based keyframe, `pointer-events: none`, and `@media (prefers-reduced-motion: reduce)`.

- [ ] **Step 2: Run tests and verify RED**

Run:

```powershell
npm --prefix web/platform test -- SessionProgressBar
```

Expected: FAIL because `SessionProgressBar` does not exist.

- [ ] **Step 3: Implement the minimal independent component**

```tsx
import styles from "./SessionProgressBar.module.css";

type SessionProgressBarProps = { label: string; visible: boolean };

export function SessionProgressBar({ label, visible }: SessionProgressBarProps) {
  if (!visible) return null;
  return (
    <div aria-label={label} className={styles.track} role="progressbar">
      <span className={styles.indicator} />
    </div>
  );
}
```

The CSS must keep the track fixed at `inset: 0 0 auto`, height `0.1875rem`, `z-index: 100`, and animate only the indicator's `transform`.

- [ ] **Step 4: Run focused tests and verify GREEN**

Run the command from Step 2. Expected: all SessionProgressBar tests PASS.

- [ ] **Step 5: Commit the component**

```powershell
git add web/platform/src/features/session/SessionProgressBar
git commit -m "feat(web): add session progress line"
```

---

### Task 2: Privacy-safe restoration shell

**Files:**
- Create: `web/platform/src/features/session/SessionRestorationShell/SessionRestorationShell.tsx`
- Create: `web/platform/src/features/session/SessionRestorationShell/SessionRestorationShell.module.css`
- Test: `web/platform/src/features/session/SessionRestorationShell/SessionRestorationShell.test.tsx`
- Modify: `web/platform/src/i18n/ru.ts`

**Interfaces:**
- Consumes: `isProgressVisible: boolean`, `isRetryableError: boolean`, `onRetry(): void`.
- Produces: `SessionRestorationShell(props): JSX.Element` and composes `SessionProgressBar`.

- [ ] **Step 1: Write failing privacy and retry tests**

```tsx
it("renders only neutral geometry while refresh is pending", () => {
  render(<SessionRestorationShell isProgressVisible={false} isRetryableError={false} onRetry={vi.fn()} />);
  expect(screen.queryByText(/Восстанавливаем сессию/i)).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "Повторить" })).not.toBeInTheDocument();
  expect(screen.getByTestId("session-restoration-shell")).toHaveAttribute("aria-busy", "true");
});

it("offers one explicit retry after a transient failure", async () => {
  const onRetry = vi.fn();
  render(<SessionRestorationShell isProgressVisible={false} isRetryableError onRetry={onRetry} />);
  expect(screen.getByRole("status")).toHaveTextContent("Не удалось восстановить соединение");
  await userEvent.click(screen.getByRole("button", { name: "Повторить" }));
  expect(onRetry).toHaveBeenCalledTimes(1);
});
```

- [ ] **Step 2: Run tests and verify RED**

```powershell
npm --prefix web/platform test -- SessionRestorationShell
```

Expected: FAIL because the shell and new dictionary keys do not exist.

- [ ] **Step 3: Add dictionary keys and implement the shell**

Add under `ru.workspace`:

```ts
sessionProgressLabel: "Восстановление сессии",
sessionRetryableError: "Не удалось восстановить соединение.",
sessionRetry: "Повторить",
```

Render a neutral shell with `aria-busy={!isRetryableError}`, an `aria-hidden` skeleton sidebar/header, and a centered `role="status"` retry surface only in the error state. Do not accept or render any authenticated data as props.

- [ ] **Step 4: Add stable responsive CSS**

Use `--sidebar-width` on desktop, switch to a zero-width sidebar below `48rem`, reserve header height without text, animate skeleton opacity only, and disable skeleton animation for reduced motion.

- [ ] **Step 5: Run focused tests and verify GREEN**

Run the command from Step 2. Expected: all SessionRestorationShell tests PASS.

- [ ] **Step 6: Commit the shell**

```powershell
git add web/platform/src/features/session/SessionRestorationShell web/platform/src/i18n/ru.ts
git commit -m "feat(web): add safe session restoration shell"
```

---

### Task 3: Delayed refresh state machine and retry

**Files:**
- Modify: `web/platform/src/features/session/SessionRefresh/SessionRefresh.tsx`
- Modify: `web/platform/src/features/session/SessionRefresh/SessionRefresh.test.tsx`
- Delete: `web/platform/src/features/session/SessionRefresh/SessionRefresh.module.css`

**Interfaces:**
- Consumes: existing `webBrowserMutation`, `useRouter`.
- Produces: exactly one active attempt with phases `pending | slow | retryable_error` and explicit `retry()`.

- [ ] **Step 1: Replace tests with the complete failing behavior matrix**

Use fake timers and controllable promises to cover:

```tsx
it("keeps progress hidden for the first 149 ms and reveals it at 150 ms", async () => { /* assert timer boundary */ });
it("refreshes the server tree after 200", async () => { /* assert router.refresh */ });
it.each([400, 401, 403])("redirects invalid session status %s to login", async (status) => { /* assert replace */ });
it.each([408, 429, 500, 503])("offers retry for transient status %s", async (status) => { /* assert button */ });
it("offers retry after a network rejection", async () => { /* assert no replace */ });
it("aborts after eight seconds and offers retry", async () => { /* assert AbortSignal and button */ });
it("runs exactly one new request when retry is clicked", async () => { /* assert call count 2 */ });
it("does not start a second request on rerender", async () => { /* assert call count 1 */ });
```

- [ ] **Step 2: Run tests and verify RED**

```powershell
npm --prefix web/platform test -- SessionRefresh
```

Expected: FAIL because the old component immediately renders visible copy, redirects every failure, and has no delay, timeout, or retry.

- [ ] **Step 3: Implement the minimal state machine**

Use constants `progressDelayMs = 150` and `refreshTimeoutMs = 8_000`, an incrementing attempt ref to ignore stale completions, and `AbortController.signal` passed to `webBrowserMutation`. Clear both timers in `finally`. Only `400`, `401`, and `403` call `router.replace("/login")`; every other failure sets `retryable_error`.

Render only:

```tsx
<SessionRestorationShell
  isProgressVisible={phase === "slow"}
  isRetryableError={phase === "retryable_error"}
  onRetry={startRefresh}
/>
```

- [ ] **Step 4: Run focused tests and verify GREEN**

Run the command from Step 2. Expected: all SessionRefresh tests PASS.

- [ ] **Step 5: Commit the state machine**

```powershell
git add web/platform/src/features/session/SessionRefresh
git commit -m "feat(web): restore sessions without blocking copy"
```

---

### Task 4: Layout regression and complete verification

**Files:**
- Modify: `web/platform/src/app/app/layout.test.tsx`

**Interfaces:**
- Consumes: unchanged `WorkspaceSession` states and the new restoration shell.
- Produces: regression proof that the server layout never leaks private children during refresh.

- [ ] **Step 1: Write the failing layout regression assertion**

Change the refresh-required test to require `data-testid="session-restoration-shell"`, no visible legacy copy, no private child, no email, no conversation title, and no balance.

- [ ] **Step 2: Run the layout test and verify RED before integration adjustment**

```powershell
npm --prefix web/platform test -- src/app/app/layout.test.tsx
```

Expected: FAIL until the test reflects and receives the new shell contract.

- [ ] **Step 3: Keep `WorkspaceLayout` state routing unchanged and remove only obsolete imports/assertions**

`refresh_required` must still return `<SessionRefresh />`; do not move authenticated children into this branch.

- [ ] **Step 4: Run platform verification**

```powershell
npm --prefix web/platform test
npm --prefix web/platform run typecheck
npm --prefix web/platform run lint
npm --prefix web/platform run build
git diff --check
```

Expected: all tests PASS, TypeScript and ESLint exit 0, Next build succeeds, and diff check is clean.

- [ ] **Step 5: Commit final regression coverage**

```powershell
git add web/platform/src/app/app/layout.test.tsx
git commit -m "test(web): protect silent session restoration"
```

---

## Self-Review

- Spec coverage: delayed progress, safe shell, retryable network failures, invalid-session login, accessibility, reduced motion, mobile geometry, timeout, and no backend changes are each assigned to a task.
- Placeholder scan: complete; every implementation and verification step is explicit.
- Type consistency: `SessionRestorationShell` receives the same three props from Task 2 through Task 3; dictionary keys and state names are identical in all tasks.
