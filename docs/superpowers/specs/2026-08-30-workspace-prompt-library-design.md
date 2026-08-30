# Workspace Prompt Library Design

## Goal

Simplify the workspace home-page prompt library section so it uses the same visual language and interaction as the existing Inspiration gallery.

## Content

- Heading: `Библиотека промптов`
- Description: `Собрали промпты для любых задач и идей`
- Remove the `Все идеи` action from this section.
- Remove the current wide split promotional card, including its category label, large editorial title, and `Посмотреть пример` link.

## Card

Render exactly one existing example from the shared Inspiration collection. Reuse `InspirationExampleCard` rather than introducing a second card implementation.

The card keeps the existing Inspiration behavior:

- it displays the example image as a portrait card;
- clicking it opens the same example-detail dialog;
- prompt copying, recreation, download, sharing, focus restoration, and keyboard closing continue to use the shared component behavior.

The workspace section selects the first item from `inspirationExamples`. If the collection is empty, the section renders its heading and description without a broken card or placeholder.

## Layout

- The heading and description remain left-aligned within the existing workspace content frame.
- The single card sits below the description and does not stretch across the full section width on desktop.
- Its desktop width is capped at `25rem`, matching a normal Inspiration gallery card.
- Below `36rem`, the card fills the available content width while preserving its `2 / 3` portrait aspect ratio.
- No replacement action is added where `Все идеи` was removed.

## Component Boundaries

- `WorkspaceLanding` owns the section heading, description, selection of the first example, and section layout.
- `InspirationExampleCard` remains the sole owner of card visuals and dialog interaction.
- `inspirationExamples` remains the shared source of example data.

## Error and Empty States

No network request is introduced. If no Inspiration example is available, omit the card and keep the section copy visible.

## Testing

Update workspace tests to verify:

- the new description is rendered;
- `Все идеи`, the old category label, the old editorial title, and `Посмотреть пример` are absent;
- exactly one shared Inspiration card is rendered;
- the card exposes the existing example interaction;
- the section and card remain responsive through focused stylesheet checks.

## Non-goals

- Adding or editing Inspiration examples.
- Changing the Inspiration gallery or its dialog.
- Adding a new navigation action to the section.
