# Floating Workspace Header Design

## Goal

Replace the full-width workspace header strip with two floating controls that
stay above the right-side scroll region: the model selector on the left and the
balance or login action on the right.

## Behaviour

- The header remains sticky to the top of the workspace scroll region.
- The header itself has zero visual height, no surface colour, border, shadow,
  or divider, so it does not push conversation content down.
- The leading and trailing controls are absolutely positioned inside the
  sticky header and remain independently interactive.
- The wrapper ignores pointer events so the visible content between the two
  controls stays selectable and interactive.
- On mobile, the leading control keeps enough inline offset for the sidebar
  toggle. The trailing control stays aligned to the opposite edge.
- Inspiration keeps its existing text title instead of the model selector.
- Authentication, balance loading, model selection, and route behaviour remain
  unchanged.

## Verification

- A stylesheet contract test verifies the zero-height transparent overlay and
  interactive floating control regions.
- Existing component tests verify route naming, model selector behaviour,
  balance, and the guest login action.
- Frontend tests, typecheck, and production build must remain green.
