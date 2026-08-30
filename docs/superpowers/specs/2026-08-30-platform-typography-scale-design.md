# Platform Typography Scale Design

## Goal

Make NeiroHub headings easier to read in Cyrillic by reducing compressed tracking and formalizing the approved H1/H2/H3 scale.

## Approved Scale

- H1/display role: `40px`, weight `600`, letter spacing `-0.01em`.
- H2/section role: `32px`, weight `600`, letter spacing `-0.01em`.
- H3/subsection role: `24px`, weight `600`, letter spacing `0`.
- Interface and body roles remain within the existing `13px`–`18px` scale and use normal/zero letter spacing.

The existing mobile H1 and H2 reductions remain in place. The H3/subsection role reduces to `22px` below `48rem`.

## Token Strategy

Keep typography role-based rather than applying styles to every literal `h1`, `h2`, or `h3` element. Existing compact card names, navigation labels, and control text remain compact even when their HTML element is a heading.

Update the shared tokens in `globals.css`:

- change `--letter-spacing-display` from `-0.03em` to `-0.01em`;
- change `--letter-spacing-section` from `-0.025em` to `-0.01em`;
- add `--font-size-subsection: 1.5rem`;
- add `--line-height-subsection: 2rem`;
- add `--letter-spacing-subsection: 0`;
- add a mobile `--font-size-subsection: 1.375rem` override;
- set the inherited interface letter spacing through `--letter-spacing-interface: 0` on `body`.

## Application

Existing display and section consumers already use the shared tokens, so the H1/H2 tracking change propagates automatically. Markdown H3 content in assistant messages adopts the new subsection role because it is a genuine third-level content heading. Compact model names, file titles, step labels, and card headings retain their current role-specific sizes.

## Testing

Update the global theme contract test to require the approved desktop and mobile tokens and reject the two old compressed values. Update the typography contract test to require assistant-message H3 headings to consume the subsection role and zero tracking.

## Non-goals

- Changing the Geist font family.
- Increasing compact card or navigation labels to `24px`.
- Changing established mobile H1/H2 sizes.
- Reworking line heights outside the new subsection role.
