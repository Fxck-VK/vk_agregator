# NeiroHub static assets

This directory contains trusted, repository-owned static media with stable
public URLs.

- `brand/`: product logos and marks shared by multiple surfaces.
- `icons/models/`: static model marks that are not interactive React icons.
- `illustrations/`: shared empty-state and onboarding artwork.
- `images/`: editorial images grouped by inspiration, models, tools and articles.
- `videos/`: approved product and editorial MP4 clips grouped by feature.

Use lowercase kebab-case names. Use SVG for trusted vector artwork, AVIF/WebP
for photos where practical, PNG only when its transparency or compatibility
is required, and MP4 for approved video clips. Interactive theme-aware icons belong in `src/components/icons`.
Feature-specific files with one owner may live beside their feature in an
`assets` directory.

Never place user uploads, generated artifacts, private storage/provider URLs,
secrets or PII here.
