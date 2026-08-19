# Conversation Message Surfaces Design

## Goal

Make the conversation read like a continuous document: user prompts remain
compact bubbles aligned to the right, while assistant replies render directly
on the page without a card surface or product-name label.

## Behaviour

- A user message uses only the width required by its content, up to a readable
  maximum, and remains aligned to the right.
- An assistant message keeps the conversation reading width but has no border,
  background, radius, or card padding.
- The assistant role label is not rendered. The user role label remains for now.
- Copy, recreate, like, dislike, retry, and typing indicators keep their current
  behaviour and remain attached to the corresponding list item.
- Empty and unavailable states remain cards; only actual assistant messages lose
  their surface.
- No API, polling, rating, idempotency, or billing behaviour changes.

## Verification

- A component test verifies that assistant text no longer includes the
  `NeiroHub` role label.
- A stylesheet test verifies compact right-aligned user bubbles and transparent,
  borderless assistant messages.
- Existing conversation tests and the full frontend verification remain green.
