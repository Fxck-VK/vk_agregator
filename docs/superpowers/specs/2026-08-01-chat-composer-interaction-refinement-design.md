# Chat Composer Interaction Refinement

Status: accepted

Date: 2026-08-01

## Goal

Make the authenticated conversation composer behave like a compact chat input: it has a fixed height, sends on Enter, preserves Shift+Enter for a new line, and shows an unobtrusive three-dot waiting state while NeiroHub is preparing a reply.

## Accepted Interaction Decisions

- The textarea has a fixed block size and cannot be resized with the mouse.
- Enter submits a non-empty draft. Shift+Enter keeps the browser's normal newline behavior. An active IME composition never submits.
- After a successful message request, the parent conversation refresh state controls a visible three-dot indicator. The indicator disappears as soon as the bounded refresh completes or stops.
- The textarea placeholder is exactly `Задайте вопрос NeiroHub`.
- The former visible accepted-message sentence is removed. Existing safe error feedback and retry/idempotency behavior remain unchanged.

## State and Accessibility

`ConversationHistory` remains the owner of the bounded reply-refresh lifecycle. It passes a dedicated `isAwaitingResponse` prop to `ConversationComposer`, rather than overloading the generic disabled state. The typing indicator has a Russian accessible label from the dictionary, uses a polite live status, and respects reduced-motion preferences.

## Verification

Component tests cover Enter submission, Shift+Enter non-submission, the new placeholder, and the waiting indicator's appearance/disappearance around the parent refresh lifecycle. Focused conversation tests, lint, typecheck, the full frontend test suite, and the existing DEV delivery gates remain required.
