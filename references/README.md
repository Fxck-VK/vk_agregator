# Local design reference library

The 24-page Magnific static and original-JavaScript snapshots, downloaded assets,
desktop/mobile interaction captures and their ZIP exports are retained in this
directory on the active development machine. They are reference material rather
than Epifera application source and are ignored by Git and Docker. No local file
is deleted by this policy.

The combined ZIP is about 303 MiB and the static ZIP about 199 MiB; neither can
be included in an ordinary GitHub Git push. Repeated captured assets would also
inflate every clone/build context. Keep these files with the local reference
delivery; do not place private browser snapshots or captures in application code.

Local entry points, when the downloaded bundles are present:

- `magnific-reference-index.html`: static, JS and captured-state navigation.
- `magnific-html/index.html`: static 24-page archive.
- `magnific-html-js/index.html`: original-JavaScript variant and known limits.
- `MAGNIFIC_REFERENCES_README.txt`: original reference-delivery notes.

The reusable Epifera homepage, SEO example, content structure and URL registries
are versioned separately under `web/platform` and `docs/research`; all website
images/videos needed at runtime are bundled under `web/platform/public/assets`.
