# Floating scroll area design

## Goal

Replace the duplicated vertical scrollbar styling with one reusable `ScrollArea` whose thumb floats above the surface without reserving layout width.

## Selected approach

Create a dependency-free React component around a native overflow viewport. The browser viewport keeps wheel, touchpad, touch, keyboard, and programmatic scrolling. The component hides only the native visual scrollbar and renders a decorative overlay track and draggable thumb whose geometry follows the viewport.

The public interface supports different semantic roots and viewports so existing landmarks, lists, dialogs, refs, test IDs, and keyboard handlers remain intact. Consumer CSS continues to own dimensions, padding, positioning, borders, and backgrounds; the shared component owns scrolling and scrollbar visuals.

## Behavior

- The track is transparent and does not occupy layout space.
- The thumb is placed over the right edge of the scroll surface, uses the NeiroHub accent palette, and has a minimum usable height.
- The thumb appears while the area is hovered, focused, scrolling, or being dragged, then fades when idle.
- The track and thumb are absent when content does not overflow.
- Clicking the track moves the viewport toward that position; dragging the thumb scrolls proportionally.
- Content-size and viewport-size changes recalculate the thumb.
- `prefers-reduced-motion: reduce` removes the fade transition.

## Scope

Migrate vertical scrolling in:

- the main workspace;
- shared modal backdrops and the chat file library;
- subscription plans;
- sidebar conversations;
- the account menu and updates feed;
- the workspace model selector;
- image template, quality, and aspect-ratio pickers;
- the inspiration thumbnail rail.

Horizontal tabs and carousels remain native and are outside this change.

## Alternatives considered

1. Shared native scrollbar CSS. Lowest complexity, but classic scrollbars may still reserve width and vary by browser or operating system.
2. A third-party scroll-area library. Mature behavior, but adds a dependency for a focused visual requirement.
3. A local overlay `ScrollArea`. Selected because it provides one appearance and true overlay behavior without a new package.

## Accessibility and performance

- The overlay track is `aria-hidden`; the native viewport remains the scrolling control.
- Existing semantic elements and focus behavior are preserved.
- Scroll updates are applied immediately for reliability and batched again with `requestAnimationFrame` for layout changes; they do not re-render consumer content.
- `ResizeObserver`, DOM mutation observation, window resize, and captured media load events keep geometry current.

## Verification

- Component tests cover overflow detection, thumb geometry, scrolling visibility, track clicks, and dragging.
- Style tests cover native scrollbar hiding, overlay placement, idle fade, brand treatment, and reduced motion.
- A consumer contract test confirms every scoped vertical area uses `ScrollArea` and no longer owns visual scrollbar rules.
- Run focused tests, the full test suite, TypeScript, ESLint, and a browser check with both the page and subscription modal scroll areas.
