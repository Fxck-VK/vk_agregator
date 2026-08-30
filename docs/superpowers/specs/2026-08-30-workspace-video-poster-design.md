# Workspace Video Poster Design

## Goal

Use the user-supplied NeiroHub artwork as the start screen for the existing workspace how-to video.

## Source Artwork

The supplied `1680 × 941` PNG is used without visual editing. It is copied into the platform's public workspace image assets as `neirohub-how-it-works-poster.png` and exposed through `assetPaths`.

The image is served from the same domain as the platform and is included in the packaged application.

## Player Behavior

- `WorkspaceLanding` passes the poster asset to the existing `VideoPlayer`.
- Before playback, the custom start overlay displays the poster across the complete `16 / 9` video frame.
- The poster uses `cover` positioning with its center preserved. Because the supplied artwork is already almost `16 / 9`, cropping is minimal.
- The existing circular Play button remains centered above the artwork.
- No additional text, caption, logo, or tint is added over the supplied image.
- Clicking Play removes the overlay, starts the video, and reveals native video controls.
- Pausing after playback has started does not restore the overlay.

## Implementation Boundary

`VideoPlayer` continues to own playback state and the start overlay. The poster URL is applied to the overlay through a scoped CSS custom property so the component does not need a second image component or duplicate layout logic. The same URL remains on the native `poster` attribute as a browser fallback.

When no poster is supplied, the current neutral gradient overlay remains unchanged.

## Error Handling

- If video playback cannot start, the poster overlay is restored.
- If the poster cannot load, the existing gradient background remains visible beneath it.
- Existing video-unavailable behavior remains unchanged.

## Testing

Update focused tests to verify:

- the public poster path is exposed by `assetPaths`;
- the poster is passed from `WorkspaceLanding` to `VideoPlayer`;
- the start overlay receives the poster URL;
- the overlay uses centered `cover` sizing;
- the Play button stays above the poster;
- the existing playback and failure behavior remains intact;
- asset validation and packaging include the PNG.

## Non-goals

- Editing or regenerating the supplied artwork.
- Changing the video file or playback controls.
- Adding text over the poster.
- Changing the dimensions of the video section.
