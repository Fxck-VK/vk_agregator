# Remove the blue NeiroHub eyebrow

## Goal

Remove the small blue `NeiroHub` label shown above page titles throughout the authenticated workspace while preserving the sidebar brand, page titles, descriptions, model selector, and unrelated blue section labels.

## Scope

- Remove the brand-only eyebrow from the models catalog header.
- Remove the brand-only eyebrow from generic workspace section headers.
- Remove the two translation values that become unused.
- Keep `Библиотека` and `Галерея NeiroHub` eyebrow labels unchanged because they are section labels rather than the standalone brand marker shown in the references.
- Keep all non-eyebrow occurrences of `NeiroHub` unchanged.

## Implementation

Delete the two `<p className={styles.eyebrow}>` elements that render `ru.modelsCatalog.eyebrow` and `ru.workspace.eyebrow`. Delete only those translation properties. Do not change CSS, spacing tokens, page titles, descriptions, or layout containers.

## Verification

- Add or update component tests to assert that the standalone `NeiroHub` eyebrow is absent while titles and descriptions remain.
- Run the focused tests, lint, typecheck, and production build.
- Review the final diff to confirm that unrelated brand copy and sidebar branding are untouched.
