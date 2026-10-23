# Magnific dual reference archive implementation plan

> Execute inline in the current authorized task. Preserve the existing static
> archive; no worktree, additional site, commit, push or deployment.

**Goal:** Keep static and original-JavaScript versions of all 24 agreed Magnific
pages, plus observed desktop interaction states and mobile views.

**Architecture:** Reuse the captured public rendered DOM and downloaded source
assets. Add a separate `references/magnific-html-js` bundle that preserves
application scripts and hydration data, with local asset URLs and source metadata.
Capture interaction states from the live public originals using the selected
browser; preserve screenshots, DOM and action/result evidence in
`references/magnific-states`. Never label source-site verification as local replay
verification. The existing in-app-browser `file://` denial remains a local visual
verification limitation, with no alternate server or browser workaround.

**Tools:** Python standard library and existing lxml for file assembly; supported
CUA browser APIs for DOM, UI actions, screenshots, viewport and asset exports.

## Constraints

- Use only the active Epifera checkout.
- Keep the 24-page source list and query-string collection ID unchanged.
- Keep original app scripts; exclude advertising/analytics bootstraps.
- Never execute downloaded JavaScript in the host Node/Python process.
- Do not submit accounts, payments, forms, uploads or generation requests.
- Do not modify Epifera application code or the existing static snapshot files.
- Restore the browser viewport after mobile capture.

## Tasks

- [x] Inventory retained application scripts, hydration data and missing resources;
  build `magnific-html-js` with script source provenance and local asset manifests.
  Verify 24 files, preserved inline initialization, local script existence and no
  `script-src 'none'` policy in the JS variant.
- [x] Capture desktop controls from each public source. Save default views,
  selected tabs, open menus, carousel/accordion states and a per-page action log.
  Only click observed reversible UI controls. Mark unavailable or server-dependent
  controls explicitly rather than claiming full offline behavior.
- [x] Capture mobile views of all 24 pages at 390×844, plus mobile navigation and
  applicable page-specific states. Save screenshots and HTML with viewport sizes.
  Restore the default viewport.
- [x] Build one navigation index linking static, JS, live source and captured
  states. Verify all file references and source counts, preserve static checksums,
  then package and CRC-check a combined ZIP. Report the local runtime verification
  restriction and actual interaction coverage.

## Verification commands

Run the task's builders and `verify_interactive_archive.py` from
`.cache/magnific-html-assets/` using the bundled Python interpreter. Run Node's
`--check` only on generated helper code if any; do not run the downloaded app.
Inspect all verification outputs and final `git status --short` before reporting.

## Result and verified limits

- Entry point: `references/magnific-reference-index.html`.
- 24 original-JS pages with startup/hydration data; 448 script references,
  443 with local source copies. Eight runtime references use the original CDN
  or source URL (including ES modules whose import base must be retained).
- 275 delivered UI states, with desktop and mobile baselines for all 24 pages.
  168 actions have explicit selected/expanded state confirmations; another 54
  public actions have captured results without an automatic assertion. No claim
  that every control or account/backend action was exercised.
- Five unstable/intermediate DOM exports were retained as superseded cache data
  and excluded from delivery. Replacement captures serialize DOM and metadata in
  one read-only evaluation, returning bounded string chunks from that one value.
- Verification passed: complete sources, matching headings, 22,516 local links,
  original startup data, valid images and 2,048 unchanged static files.
- Combined ZIP: `references/magnific-references-static-js-states.zip`,
  317,389,918 bytes, 3,110 files; ZIP CRC check passed.
- Local application startup/hydration remains unverified because the in-app
  browser rejects `file://`. Source-site interaction evidence is separate from
  local runtime verification. No workaround server, alternate browser, worktree,
  commit, push or deployment was used.
