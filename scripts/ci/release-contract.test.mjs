import assert from 'node:assert/strict';
import { readFileSync, readdirSync } from 'node:fs';
import test from 'node:test';
import { resolve } from 'node:path';
import { validateReleaseContract } from './check-release-contract.mjs';

const root = resolve(import.meta.dirname, '../..');
const files = new Map([
  '.github/workflows/ci.yml', '.github/workflows/docker-images.yml',
  '.github/workflows/deploy-dev.yml', '.github/workflows/nightly-quality.yml',
  'scripts/deploy/verify-release-images.sh', 'docker-compose.prod.yml', 'docker-compose.dev-web.yml',
  'scripts/ci/dev-deploy-preflight.ps1',
].map(p => [p, readFileSync(resolve(root, p), 'utf8')]));
const dockerfiles = readdirSync(root).filter(p => p.startsWith('Dockerfile.'));
const check = (changes = {}) => validateReleaseContract(p => changes[p] ?? files.get(p), dockerfiles);
function rejects(path, before, after, message) {
  assert(files.get(path).includes(before), 'mutation must change actual source');
  assert.throws(() => check({[path]: files.get(path).replace(before, after)}), message);
}
test('current source agrees across publication, verifier, Compose and DEV', () => {
  assert.equal(check().services.length, 7);
});
test('removing an image from either publication matrix is rejected', () => {
  rejects('.github/workflows/docker-images.yml', '- service: api', '- service: unrelated', /inventory|mapping/i);
});
test('a verifier expecting a different image set is rejected', () => {
  rejects('scripts/deploy/verify-release-images.sh', 'services=(api worker', 'services=(worker', /inventory/i);
});
test('certificate names must come from the exact signed source', () => {
  rejects('scripts/deploy/verify-release-images.sh', '--certificate-github-workflow-name "${workflow_name}"', '--certificate-github-workflow-name "Docker Images"', /certificate/i);
});
test('rollback must fetch source history before signature validation', () => {
  rejects('.github/workflows/deploy-dev.yml', 'fetch-depth: 0', 'fetch-depth: 1', /history/i);
});
test('publication passes the exact source commit into the reusable rollout', () => {
  rejects('.github/workflows/docker-images.yml', 'source_sha: ${{ github.sha }}', 'source_sha: dev-deploy', /exact.*SHA/i);
});
test('successful exact-SHA CI is a required publication dependency', () => {
  rejects('.github/workflows/docker-images.yml', 'needs: [validate_source, quality-gate]', 'needs: validate_source', /quality gate/i);
});
test('deployment verifies signatures before preparing SSH', () => {
  rejects('.github/workflows/deploy-dev.yml', '- name: Verify signed DEV GHCR images', '- name: Disabled image verification', /signature/i);
});
test('deployment smoke cannot disappear', () => {
  rejects('.github/workflows/deploy-dev.yml', '- name: Run DEV smoke', '- name: Disabled smoke', /smoke/i);
});
test('unknown workflow certificate name fails before push', () => {
  rejects('.github/workflows/docker-images.yml', 'name: Release Images', 'name: Unknown Images', /workflow name/i);
});

test('security and infrastructure admission precede expensive source tests', () => {
  const source = files.get('scripts/ci/dev-deploy-preflight.ps1');
  const stages = source.match(/^\$stageNames = @\(([^\n]+)\)/m)?.[1];
  assert(stages, 'Preflight stages must be declared explicitly');
  for (const early of ['audit', 'govulncheck', 'infrastructure', 'trivy']) {
    assert(stages.includes(`"${early}"`) && stages.includes('"tests"'), 'Required admission stage is missing');
    assert(stages.indexOf(`"${early}"`) < stages.indexOf('"tests"'), `${early} must fail before lengthy source tests`);
  }
});

test('early source checks are the mandatory first full preflight stage', () => {
  rejects('scripts/ci/dev-deploy-preflight.ps1', '@("source-fast", "release-contract"', '@("release-contract"', /source.fast|early source/i);
});

test('preparation cannot capture an inventory before early source checks', () => {
  rejects('scripts/ci/dev-deploy-preflight.ps1', '& pwsh -NoProfile -File scripts/ci/release-source-check.ps1 -BaseRef $BaseRef', 'Write-Host "source check removed"', /preparation|early source/i);
});

test('CI keeps the early source selector regressions', () => {
  rejects('.github/workflows/ci.yml', ' scripts/ci/release-source-check.test.mjs', '', /early source/i);
});
