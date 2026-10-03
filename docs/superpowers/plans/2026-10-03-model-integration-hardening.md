# Model integration corrections for DEV

Scope: current 103-model catalog at 6dd08963; fix documented discrepancies,
run offline checks, commit one logical change, push fastlife_dev/dev-deploy and
verify the exact-SHA GitHub Actions DEV deployment and smoke. No paid agent
generations, production rollout, secret changes or fabricated admission evidence.

1. Preserve the current tracked source ZIP and previous Mini App patch/stash.
2. Restrict public operational image limits: Nano Banana 2/Pro and GPT Image 2
   return one image; GPT Image 2 permits 15 references. Add adapter rejection,
   public catalog/resolver boundary tests and preserve frozen admission bytes.
3. Correct PoYo wire fields, aspect ratios, Seedream/Runway/Seedance prompt bounds
   and expose/enforce the same Unicode character bounds before job creation.
4. Align Gemini reference byte limits to the existing 20 MiB upload limit and
   pending video reference metadata to the implemented PNG/JPEG transport.
5. Validate provider readiness against the worker provider selection; preserve
   explicit provider flags and existing fail-closed pricing/admission gates.
6. Remove unsupported Runway resolution control from provider payload; expose
   only an evidenced application option or keep the uncertain request closed.
   Unverified VEO IDs and KIE native output-cap behavior remain honest DEV manual
   checks, with existing admission/pricing/output-validation gates intact.
7. Regenerate and validate all 103 preview rows. Run relevant Go tests/vet,
   web lint/typecheck/tests/build/packaging, onboarding check and DEV preflight.
8. Review the integrated patch, commit, fast-forward push both requested DEV
   branches and watch CI -> signed Docker Images -> Deploy DEV -> smoke.

Review focus: rejection before reservation/provider submit; byte versus character
limits; no stale UI count/reference options; provider chain omissions; preservation
of owner checks, ledger capture/release, retry/idempotency and moderation.

Execution ledger:
- Backup created: source-backup-103-before-fixes-20261003.zip outside repository.
- Prior Mini App edits preserved in patch and stash; workspace switched to the
  current source. Main checkout and untracked output/ left untouched.
- Ruling: tighten operational public projections and adapter validation without
  renewing frozen legacy fingerprints or pretending successful live admission.
  Narrowing unsupported requests does not authorize new capabilities.
- RED/GREEN: adapter counts/references/20 MiB bytes, public admission-preserving
  limits, stale resolver counts, Unicode Seedream boundary, catalog prompt bounds
  and transport metadata reproduced failures, then passed with corrections.
- PoYo wire and prompt boundaries, config worker-set readiness and frontend
  prompt/automatic-resolution controls have independent RED/GREEN checks.
- Ruling: Runway resolution is provider-selected; preserve the existing internal
  default price key, hide the unsupported resolution choice and reject other
  public resolution requests. Do not assert fixed output dimensions.
- Review corrections: GPT Image 2 aggregate references are limited to the
  existing 256 MiB adapter budget in catalog, frontend and owned-artifact
  validation before job creation. The legacy video DTO mirrors automatic
  resolution and prompt bounds; nonempty short prompts explain the rejection.
- Full Go tests/vet and the initial platform lint/typecheck/build/packaging,
  1622 platform tests and assets passed. Generated preview still has 103 IDs.
- Security preflight exposed stale brace-expansion, the unpatched braces lint
  dependency, OpenTelemetry exporters and the local Go toolchain. These must
  be corrected and the full exact-commit preflight must pass before push.
- Security corrections: brace-expansion 5.0.12 for Mini App/Admin; platform
  Next lint keeps all current rules with a scoped fast-glob alias to official
  glob 12.0.0 and a canonical exact root directory. Its contract test rejects
  future broader caller use and proves internal-anchor lint still works.
  All three npm audits now report zero vulnerabilities.
- Go baseline/build images pinned to 1.25.14; OpenTelemetry family updated to
  1.45.0 and compress to 1.18.7. Symbol govulncheck and pinned Trivy passed.
- Further review: Mini App rejects the unsupported Runway resolution tariff
  before estimate/job creation and hides native-resolution controls. Catalog
  also clamps stale reference controls; preview regenerated after corrections.
- PoYo also rejects legacy queued Runway non-default input tariffs before HTTP,
  retaining the worker's existing terminal-failure release behavior and output
  probe bounds. Regression reproduced an accepted unsupported request, then
  passed. Full Go tests/vet, focused Mini App video tests/typecheck/lint and the
  final platform production build/packaging passed; review found no open P1/P2.
