# Shared Input Control Chip Implementation Plan

**Goal:** Give every compact control inside the universal composer one shared visual shell without changing its behavior.

## Implementation

1. Add a reusable `InputControlChip` component with button and group modes.
2. Move shared height, border, radius, spacing, typography, hover, focus, and disabled styles into that component.
3. Use it for media upload, aspect ratio, quality, and output count controls.
4. Keep selector panels and the output-count `− / +` behavior inside their existing feature components.
5. Verify the shared contract with component and composer tests, then run typecheck and the targeted test suite.

