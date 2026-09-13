# Model Selector Sections Design

## Goal

Split the scrollable model list inside `WorkspaceModelSelector` into five ordered sections: `Популярные`, `Изображения`, `Текст`, `Видео`, and `Аудио`.

## Structure and data

- Keep the search field above the scrollable content and the catalogue link below it.
- Keep all five section headings inside the existing `ScrollArea` in the requested order.
- Treat the first two entries of the image-model catalogue as popular because the current catalogue order is the product's ranking signal.
- Put the remaining image-model entries in the `Изображения` section so every selectable model appears exactly once.
- Keep `Текст`, `Видео`, and `Аудио` visible as prepared empty sections until their catalogues exist; show the compact copy `Скоро появятся` below each empty heading.
- Do not invent models or add routes for unsupported generator types.
- Continue navigating every selectable model to `/app/image?model=<id>`.

## Search behavior

- Filter the real catalogue items with the existing name-and-id matching behavior.
- Preserve each matching model's original section assignment; filtering must not promote an image model into `Популярные`.
- When no real models match, replace the section stack with the existing global empty-search message.

## Presentation and accessibility

- Use semantic section headings and give each populated model list an accessible name matching its heading.
- Keep the current model row, selected state, animation, scrollbar clearance, and inset catalogue-link hover unchanged.
- Separate sections with the shared spacing tokens and style empty-section copy with the existing muted text tokens.

## Rejected alternatives

- Category tabs would hide the requested sections instead of letting them live together in one panel.
- Duplicating popular models again under `Изображения` would create duplicate controls with the same model name and selection action.
- Static placeholder model cards would misrepresent unavailable product capabilities.

## Verification

- A component test must require all five headings in the requested order.
- A component test must prove that the first two catalogue models render only under `Популярные` and later image models render only under `Изображения`.
- A component test must require the compact empty state under `Текст`, `Видео`, and `Аудио`.
- Existing selection, search, close-animation, and navigation tests must continue passing.
- A style contract must require a vertically spaced section stack and compact muted empty-section copy.
- The panel must be inspected in the local browser with its real six-model preview catalogue.

