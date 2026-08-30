# Workspace NeiroHub FAQ Design

## Goal

Replace the workspace FAQ copy with the five approved NeiroHub questions and align the accordion rows with the supplied rounded StudyAI reference.

## Questions and Answers

1. `Что такое NeiroHub?` — explain that NeiroHub combines AI conversations, image generation, model selection, and saved results in one workspace.
2. `Что такое собственные нейросети NeiroHub?` — clarify that the phrase refers to the models available through NeiroHub's unified interface, with visible capabilities, parameters, and launch prices; do not claim that NeiroHub trained every model.
3. `Что такое токены и подписка?` — explain that stars are the platform balance units used for launches and that a separate subscription is not currently required for basic use.
4. `Как купить подписку?` — state that subscriptions are not currently sold and direct users to profile/balance replenishment for paid launches.
5. `Есть ли бесплатный доступ?` — explain that browsing the workspace and catalogue is free while paid model launches require sufficient balance.

## Interaction

Keep the existing native `<details>` accordion behavior. Every row remains keyboard accessible, the answer expands below the question, and the right-side chevron rotates when opened.

## Visual Treatment

- One full-width rounded row per question within the existing content frame.
- `1.5rem` corner radius, no visible default border, and the shared surface background.
- Minimum row height `5rem` with `1.25rem 1.5rem` padding.
- Question text uses the existing supporting size and semibold weight.
- Rows keep the current spacing and receive a subtle border only on hover or keyboard focus.

## Testing

Workspace rendering tests verify all five questions, exactly five FAQ rows, and absence of the four removed questions. Stylesheet tests verify the row radius, minimum height, transparent border, supporting type role, and hover/focus border.

## Non-goals

- Adding billing or subscription functionality.
- Changing the FAQ heading.
- Replacing native `<details>` with custom JavaScript state.
