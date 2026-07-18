# Production Artifact Scanner Bypass Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow an explicitly acknowledged production deployment without OpenAI artifact scanning while preserving fail-closed defaults.

**Architecture:** Runtime validation and both deploy preflight implementations will accept `ARTIFACT_SCANNER=none` only when `ALLOW_UNSCANNED_ARTIFACTS_IN_PRODUCTION=true`. OpenAI scanning continues to require `OPENAI_API_KEY`. Production secrets will opt into the bypass explicitly.

**Tech Stack:** Go configuration validation, Bash and PowerShell deploy preflight, GitHub Actions, split production env secrets.

## Global Constraints

- The bypass is disabled by default.
- `ARTIFACT_SCANNER=openai` still requires `OPENAI_API_KEY`.
- Provider routing, billing, ownership, storage, and delivery behavior must not change.
- Secret values must never be printed or committed.

---

### Task 1: Runtime Configuration Contract

**Files:**
- Modify: `internal/platform/config/config_test.go`
- Modify: `internal/platform/config/config.go`

**Interfaces:**
- Consumes: `Config.ArtifactScanner` and `Config.AllowUnscannedArtifactsInProduction`.
- Produces: production validation that permits only the explicit bypass.

- [ ] **Step 1: Change the existing regression test to require explicit bypass acceptance**

```go
func TestValidateProductionExplicitFlagAllowsDisabledArtifactScanner(t *testing.T) {
	cfg := productionDeepInfraConfig()
	cfg.ArtifactScanner = "none"
	cfg.OpenAIAPIKey = ""
	cfg.AllowUnscannedArtifactsInProduction = true

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}
```

- [ ] **Step 2: Run the test and verify RED**

Run: `go test ./internal/platform/config -run 'TestValidateProduction(RequiresArtifactScanner|ExplicitFlagAllowsDisabledArtifactScanner)'`

Expected: the explicit bypass test fails with `ARTIFACT_SCANNER=openai is required in production`.

- [ ] **Step 3: Honor the explicit flag in runtime validation**

```go
if c.usesRealGenerationProvider() && artifactScannerDisabled(c.ArtifactScanner) && !c.AllowUnscannedArtifactsInProduction {
	return fmt.Errorf("config: ARTIFACT_SCANNER=openai is required in production unless ALLOW_UNSCANNED_ARTIFACTS_IN_PRODUCTION=true")
}
```

- [ ] **Step 4: Run the focused config tests and verify GREEN**

Run: `go test ./internal/platform/config`

Expected: PASS.

### Task 2: Deploy Preflight Parity

**Files:**
- Create: `scripts/deploy/test-prod-env.sh`
- Modify: `scripts/deploy/check-prod-env.sh`
- Modify: `scripts/deploy/check-prod-env.ps1`
- Modify: `.github/workflows/ci.yml`

**Interfaces:**
- Consumes: the same two env variables as runtime validation.
- Produces: matching Bash and PowerShell preflight behavior plus a CI regression test.

- [ ] **Step 1: Add a behavioral test with three cases**

The fixture uses fake values and asserts:

```bash
expect_failure "scanner none without bypass" bash scripts/deploy/check-prod-env.sh --env-file "${env_file}"
printf '%s\n' 'ALLOW_UNSCANNED_ARTIFACTS_IN_PRODUCTION=true' >> "${env_file}"
bash scripts/deploy/check-prod-env.sh --env-file "${env_file}"
sed -i 's/ARTIFACT_SCANNER=none/ARTIFACT_SCANNER=openai/' "${env_file}"
expect_failure "OpenAI scanner without key" bash scripts/deploy/check-prod-env.sh --env-file "${env_file}"
```

The same fixture is passed to `pwsh -File scripts/deploy/check-prod-env.ps1` when `pwsh` is available.

- [ ] **Step 2: Run the new test and verify RED**

Run: `bash scripts/deploy/test-prod-env.sh`

Expected: explicit bypass case fails before implementation.

- [ ] **Step 3: Implement matching Bash and PowerShell conditions**

```bash
if [[ "${app_env}" == "production" && ( -z "${artifact_scanner}" || "${artifact_scanner}" == "none" ) ]] &&
   ! is_true_value "$(get_value ALLOW_UNSCANNED_ARTIFACTS_IN_PRODUCTION false)"; then
  add_problem ARTIFACT_SCANNER "must be openai in production unless ALLOW_UNSCANNED_ARTIFACTS_IN_PRODUCTION=true"
fi
```

```powershell
if ($appEnv -eq "production" -and @("", "none") -contains $artifactScanner -and
    -not (Is-TrueValue (Get-Value -Values $envValues -Name "ALLOW_UNSCANNED_ARTIFACTS_IN_PRODUCTION" -Default "false"))) {
    Add-Problem -Problems $problems -Name "ARTIFACT_SCANNER" -Reason "must be openai in production unless ALLOW_UNSCANNED_ARTIFACTS_IN_PRODUCTION=true"
}
```

- [ ] **Step 4: Add `bash scripts/deploy/test-prod-env.sh` to the CI infrastructure job**

Expected: CI tests production preflight behavior on every push and pull request.

- [ ] **Step 5: Run deploy tests and syntax checks**

Run: `bash -n scripts/deploy/check-prod-env.sh scripts/deploy/test-prod-env.sh`

Run: `bash scripts/deploy/test-prod-env.sh`

Run: `powershell -NoProfile -ExecutionPolicy Bypass -File scripts/ci/validate-infra.ps1`

Expected: PASS with non-empty output.

### Task 3: Production Secret And Full Verification

**Files:**
- No repository file contains production secret values.
- Update GitHub secret: `ENV_SECRETS_PROD`.

**Interfaces:**
- Consumes: canonical production secret fragment.
- Produces: explicit scanner bypass without `OPENAI_API_KEY`.

- [ ] **Step 1: Rebuild `ENV_SECRETS_PROD` without printing values**

Set exactly:

```env
ARTIFACT_SCANNER=none
ALLOW_UNSCANNED_ARTIFACTS_IN_PRODUCTION=true
```

Remove `OPENAI_API_KEY` from that fragment and reject duplicate keys before upload.

- [ ] **Step 2: Run repository verification**

Run: `gofmt -l .`

Run: `go test ./...`

Run: `go vet ./...`

Run: `git diff --check`

Expected: all commands exit 0 and `gofmt -l .` is empty.

- [ ] **Step 3: Commit implementation, push `serega`, and deploy only after the normal protected-main flow**

The production workflow must deploy an image built from the exact merged `main` SHA. Verify public health and that `/admin`, `/metrics`, and `/debug` remain closed.
