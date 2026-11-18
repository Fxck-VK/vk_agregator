# Picomatch security backport for Epifera

The source is the official MIT-licensed [picomatch 4.0.7](https://github.com/micromatch/picomatch/tree/4.0.7), npm tarball `https://registry.npmjs.org/picomatch/-/picomatch-4.0.7.tgz` from upstream commit `6bb40679c218cefba5d4e9662408c5fdf699a8bb`. Its npm integrity is:

```text
sha512-qcJu88Q2IWqJsDD529JKMdwGm/dvInW4HvQnRwiH9JtihJvzGOscDtHE3x1pBKeUOTysQ8kVmLnJ2kJu7yhcGA==
```

The only executable change is the guard at `extglobOpen` in `lib/parse.js`. Actual star and plus extglob tokens now raise `SyntaxError` before opening a regular expression group, regardless of `maxExtglobRecursion`. This intentionally restricts the build tools' pattern grammar to address [GHSA-fc5q-39xc-vjpf](https://github.com/advisories/GHSA-fc5q-39xc-vjpf). Standard globs and bounded extglobs retain upstream behavior. The upstream license, README and all other runtime files are intact.

The archive is a local build dependency. Regenerate and verify it from the repository root with Node.js and npm 10.9.2:

```sh
node --test web/vendor/picomatch/backport.test.cjs
tmp="$(mktemp -d)"
packed="$(npm pack --ignore-scripts --silent ./web/vendor/picomatch --pack-destination "$tmp")"
cmp "$tmp/$packed" web/vendor/epifera-picomatch-4.0.7-epifera.1.tgz
```

When upstream publishes an official fix for this advisory, remove this backport after checking both web applications with the official version.
