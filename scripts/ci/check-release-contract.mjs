import assert from 'node:assert/strict';
import { readFileSync, readdirSync } from 'node:fs';
import { resolve } from 'node:path';
import { pathToFileURL } from 'node:url';

// These checks deliberately recognize this repository's literal declarations,
// not arbitrary YAML/Bash. A structural change must update the contract/tests.
function job(source, id) {
  const start = source.indexOf(`\n  ${id}:`);
  assert(start >= 0, `Missing workflow job: ${id}`);
  const rest = source.slice(start + 1);
  const end = rest.slice(1).search(/\n  [a-zA-Z0-9_-]+:/);
  return end < 0 ? rest : rest.slice(0, end + 1);
}
const inventory = values => [...new Set(values)].sort();
function matrix(source, expected, label) {
  const entries = [...source.matchAll(/^\s+- service: ([a-z0-9-]+)\s*\r?\n\s+dockerfile: (Dockerfile\.[a-z0-9-]+)/gm)];
  assert.equal(entries.length, expected.length, `${label} inventory must list every image once`);
  assert.equal([...source.matchAll(/^\s+- service:/gm)].length, entries.length, `${label} mapping must be literal`);
  assert.deepEqual(inventory(entries.map(m => m[1])), expected, `${label} inventory differs from Dockerfiles`);
  for (const [, service, dockerfile] of entries) assert.equal(dockerfile, `Dockerfile.${service}`, `${label} image/Dockerfile mapping differs`);
}
export function validateReleaseContract(read, dockerfiles) {
  const services = inventory(dockerfiles.map(p => p.replace(/^Dockerfile\./, '')));
  assert(services.length > 0, 'Release image inventory is empty');
  const images = read('.github/workflows/docker-images.yml');
  const names = [...images.matchAll(/^name: (.+)\r?$/gm)];
  assert.equal(names.length, 1, 'Exactly one literal workflow name is required');
  const workflowName = names[0][1].trim();
  assert(['Docker Images', 'Release Images'].includes(workflowName), 'Unsupported signed image workflow name');
  matrix(job(images, 'pull-request-build'), services, 'PR build');
  const publish = job(images, 'publish');
  matrix(publish, services, 'Signed publication');
  assert(publish.includes('needs: [validate_source, quality-gate]'), 'Publication must require the exact-SHA quality gate');
  const gate = job(images, 'quality-gate');
  assert(gate.includes('--workflow ci.yml') && gate.includes('--sha "${GITHUB_SHA}"'), 'Quality gate must check exact-SHA CI');
  const deployCall = job(images, 'deploy-dev');
  assert(deployCall.includes('needs: [quality-gate, publish]') && deployCall.includes('uses: ./.github/workflows/deploy-dev.yml'), 'DEV must reuse the tested publication workflow');
  assert(deployCall.includes('source_sha: ${{ github.sha }}'), 'Reusable deployment must receive exact source SHA');
  matrix(read('.github/workflows/nightly-quality.yml'), services, 'Nightly image scan');
  const compose = read('docker-compose.prod.yml') + '\n' + read('docker-compose.dev-web.yml');
  assert.deepEqual(inventory([...compose.matchAll(/dockerfile:\s*(Dockerfile\.[a-z0-9-]+)/g)].map(m => m[1].slice(11))), services, 'Compose image inventory differs');
  const verifier = read('scripts/deploy/verify-release-images.sh');
  const declared = verifier.match(/^services=\(([a-z0-9 -]+)\)/m);
  assert(declared, 'Verifier image inventory must be explicit');
  assert.deepEqual(inventory(declared[1].trim().split(/\s+/)), services, 'Verifier image inventory differs');
  assert(verifier.includes('git show "${revision}:.github/workflows/docker-images.yml"') && verifier.includes('--certificate-github-workflow-name "${workflow_name}"'), 'Certificate name must come from the exact source revision');
  for (const flag of ['--certificate-identity', '--certificate-oidc-issuer', '--certificate-github-workflow-ref', '--certificate-github-workflow-repository', '--certificate-github-workflow-sha']) {
    assert(verifier.includes(flag), `Missing signature requirement ${flag}`);
  }
  const dev = read('.github/workflows/deploy-dev.yml');
  const rollout = job(dev, 'deploy');
  assert(rollout.includes('fetch-depth: 0'), 'Rollback signature verification requires source history');
  assert(rollout.includes('ref: ${{ needs.validate_source.outputs.sha }}'), 'Rollout must checkout the exact validated SHA');
  const verify = rollout.indexOf('- name: Verify signed DEV GHCR images');
  const ssh = rollout.indexOf('- name: Prepare SSH');
  assert(verify >= 0 && ssh > verify && rollout.includes('bash scripts/deploy/verify-release-images.sh'), 'Independent signature verification must run before SSH');
  assert(rollout.includes('- name: Run DEV smoke') && rollout.includes('bash scripts/deploy/smoke-dev.sh'), 'Deployment smoke is mandatory');
  assert(rollout.includes('group: dev-deploy') && rollout.includes('cancel-in-progress: false'), 'Mutating DEV rollout must remain serialized');
  assert(read('.github/workflows/ci.yml').includes('./scripts/ci/validate-release-contract.ps1'), 'CI must execute the same release contract as local preflight');
  return { workflowName, services };
}
if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  try {
    const root = resolve(import.meta.dirname, '../..');
    const result = validateReleaseContract(p => readFileSync(resolve(root, p), 'utf8'), readdirSync(root).filter(p => p.startsWith('Dockerfile.')));
    console.log(`Release contract OK: ${result.workflowName}; ${result.services.length} exact-source images; CI, signatures, rollout and smoke agree.`);
  } catch (error) { console.error(error.message); process.exitCode = 1; }
}
