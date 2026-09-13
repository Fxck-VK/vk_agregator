# Chat Input Auto-Resize Design

## Goal

Make the shared chat textarea grow with its content through nine visible text rows, then keep its height fixed and expose the shared vertical scrollbar. Add a manual expand/collapse control that enlarges the writing area without changing the draft or caret.

## Component design

`ChatTextInput` owns textarea measurement, automatic height, and manual expansion because every composer already consumes this shared component. The textarea remains the semantic `ScrollArea` viewport, so keyboard submission, native selection, and the existing custom scrollbar continue to work.

The component measures `scrollHeight` after each value or layout change. In automatic mode it clamps the viewport height between its configured minimum and nine computed line boxes. In manual mode it uses a responsive large height and allows scrolling whenever content exceeds that height.

An accessible button is placed in the upper-right corner of the input viewport. It uses a small inline line icon, changes between expand and collapse states, and follows the existing transparent button and branded-outline interaction style. The input reserves space for both this button and the scrollbar so neither can cover text.

## Behaviour

- Empty and short drafts keep the current compact baseline.
- Content grows the input one visual line at a time, up to nine lines.
- Line ten and later scroll inside the textarea through the shared `ScrollArea`.
- The expand control switches to a responsive tall writing area; the collapse control returns to content-sized automatic mode.
- Draft value, selection, focus, Enter-to-send, Shift+Enter, IME composition, and disabled behaviour are preserved.

## Verification

Component tests cover clamping, manual expansion/collapse, accessible labels, and preserved submission behaviour. Style contract tests cover the nine-line limit and reserved inline space. The affected composer suites, typecheck, lint, and a browser overflow check complete verification.
