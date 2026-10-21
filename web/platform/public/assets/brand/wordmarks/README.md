# Brand lettering

`epifera.svg` contains the lettering and i dot extracted from the owner's
supplied vector file, preserved at `../logos/epifera-logo.svg`. The supplied
path geometry is unchanged. Letter fill uses `currentColor` for theme support.
The matching mark is `../marks/epifera-mark.svg`.

The SVG is transparent. Letter paths use `currentColor` for the surrounding
theme; the i dot has a local pink/lilac/cyan gradient. The reusable `wordmark`
group is referenced by the shared `BrandName` component, which preserves the
configured name as accessible text. There are no scripts, raster images,
external resources or duplicate visible text.

Artwork paths and dimensions are registered in `branding/brand.json` under a key
matching the derived brand file prefix. Renaming the brand selects matching
artwork or falls back to text; it never keeps a previous brand's lettering.
