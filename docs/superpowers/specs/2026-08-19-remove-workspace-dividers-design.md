# Remove Workspace Dividers Design

## Goal

Remove three decorative divider lines from the workspace so the sidebar, sticky header, conversation area, and composer read as one continuous surface.

## Scope

- Remove the right-side divider from the sidebar panel.
- Remove the bottom divider from the sticky workspace header.
- Remove the top divider from the conversation composer dock.

## Non-goals

- Do not change layout dimensions or positioning.
- Do not change sticky or scrolling behavior.
- Do not remove borders from cards, inputs, balance controls, or the account section.
- Do not change colors, typography, or component structure.

## Verification

- Add a focused regression test that inspects only the three target CSS class blocks.
- Run the focused test before and after the CSS change to demonstrate red-green behavior.
- Run the web platform test and build checks after implementation.
