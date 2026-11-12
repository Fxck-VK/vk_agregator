import assert from 'node:assert/strict';
import { readFileSync, readdirSync } from 'node:fs';
import test from 'node:test';
import { resolve } from 'node:path';
import { validateReleaseContract } from './check-release-contract.mjs';

const root = resolve(import.meta.dirname, '../..');
const dockerfiles = readdirSync(root).filter(p => p.startsWith('Dockerfile.'));
const files = new Map([
  'go.mod',
  '.github/workflows/ci.yml', '.github/workflows/docker-images.yml',
  '.github/workflows/deploy-dev.yml', '.github/workflows/nightly-quality.yml',
  'scripts/deploy/verify-release-images.sh', 'docker-compose.prod.yml', 'docker-compose.dev-web.yml',
  'scripts/ci/dev-deploy-preflight.ps1',
  ...dockerfiles,
].map(p => [p, readFileSync(resolve(root, p), 'utf8')]));
const check = (changes = {}) => validateReleaseContract(p => changes[p] ?? files.get(p), dockerfiles);
function rejects(path, before, after, message) {
  assert(files.get(path).includes(before), 'mutation must change actual source');
  assert.throws(() => check({[path]: files.get(path).replace(before, after)}), message);
}
test('current source agrees across publication, verifier, Compose and DEV', () => {
  assert.equal(check().services.length, 7);
});

test('Go module and every Go image builder reject the vulnerable HTTP server toolchain', () => {
  for (const version of ['1.26.10', '1.27.0', '1.27.3']) {
    assert.throws(() => check({'go.mod': files.get('go.mod').replace(/^go \S+/m, `go ${version}`)}), /patched Go toolchain/i);
  }
  for (const path of dockerfiles.filter(path => path !== 'Dockerfile.platform')) {
    const source = files.get(path);
    assert.throws(() => check({[path]: source.replace(/golang:\d+\.\d+\.\d+-alpine@sha256:[a-f0-9]{64}/,
      'golang:1.26.10-alpine@sha256:9ce231b28998b32e116927c61aa5c406183061d4f3f2d61b292c88e7f6bdf7ca')}), /patched Go toolchain/i);
  }
});

test('the reviewed dependency baseline rejects x/net before its HTML denial-of-service fix', () => {
  for (const version of ['0.61.0', '0.60.0']) {
    const downgraded = files.get('go.mod').replace(/golang\.org\/x\/net v\d+\.\d+\.\d+/, `golang.org/x/net v${version}`);
    assert.throws(() => check({'go.mod': downgraded}), /patched x\/net/i);
  }
});

test('media runtime images reject vulnerable or missing libexpat floors', () => {
  for (const path of ['Dockerfile.api', 'Dockerfile.worker']) {
    const source = files.get(path);
    const floor = /'libexpat>=[^']+'/;
    assert.match(source, floor, 'mutation must target the installed package floor');
    for (const replacement of ["'libexpat>=2.9.0-r0'", '']) {
      assert.throws(() => check({ [path]: source.replace(floor, replacement) }), /patched libexpat/);
    }
  }
});

test('every final runtime installs the branch-specific patched musl and utilities', () => {
  for (const path of dockerfiles) {
    const source = files.get(path);
    const fixed = path === 'Dockerfile.platform' ? '1.2.6-r3' : path === 'Dockerfile.backup' ? '1.2.5-r24' : '1.2.5-r13';
    for (const pkg of ['musl', 'musl-utils']) {
      const floor = `'${pkg}>=${fixed}'`;
      assert(source.includes(floor), `${path} must install patched ${pkg}`);
      for (const replacement of ['', `'${pkg}>=1.2.5-r12'`]) {
        assert.throws(() => check({[path]: source.replace(floor, replacement)}), /patched musl/i);
      }
    }
  }
});

test('each web checks stage must bootstrap its own lockfile-scoped dependencies', () => {
  for (const pkg of ['admin', 'platform']) {
    rejects('scripts/ci/dev-deploy-preflight.ps1',
      `Install-NpmDependenciesIfNeeded -PackagePath "web/${pkg}"`, '', /bootstrap/i);
  }
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
    for (const final of ['backend-tests', 'admin-tests', 'admin-build', 'platform-tests', 'platform-build']) {
      assert(stages.includes(`"${early}"`) && stages.includes(`"${final}"`), 'Required admission/final stage is missing');
      assert(stages.indexOf(`"${early}"`) < stages.indexOf(`"${final}"`), `${early} must fail before ${final}`);
    }
  }
});

test('each final web suite has one independently cached invocation', () => {
  const source = files.get('scripts/ci/dev-deploy-preflight.ps1');
  for (const pkg of ['admin', 'platform']) {
    const invocation = `Invoke-Native -Command $npmCommand -Arguments @("--prefix", "web/${pkg}", "run", "test")`;
    assert.equal(source.split(invocation).length - 1, 1, `Exactly one full ${pkg} suite is required`);
    assert.match(source, new RegExp(`"${pkg}-tests"\\s*\\{`));
    assert.match(source, new RegExp(`"${pkg}-build"\\s*\\{`));
  }
});

test('removing a split test/build stage cannot produce a complete receipt', () => {
  rejects('scripts/ci/dev-deploy-preflight.ps1', ', "platform-tests"', '', /stage|inventory/i);
});

test('a declared test stage cannot silently replace or lose its full suite', () => {
  rejects('scripts/ci/dev-deploy-preflight.ps1', '@("--prefix", "web/platform", "run", "test")',
    '@("--prefix", "web/platform", "run", "lint")', /suite|invocation/i);
  rejects('scripts/ci/dev-deploy-preflight.ps1', '"platform-tests" {', '"unused-platform-tests" {', /stage|suite/i);
});

test('CI runs the same Linux release guard before publication', () => {
  rejects('.github/workflows/ci.yml', 'node scripts/ci/release-linux-check.mjs', 'echo parity-removed', /Linux/i);
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

test('CI separates privileged package transactions from retryable browser downloads', () => {
  const source = files.get('.github/workflows/ci.yml').replaceAll('\r\n', '\n');
  const step = name => {
    const marker = `      - name: ${name}\n`;
    const start = source.indexOf(marker);
    assert(start >= 0, `Missing CI step: ${name}`);
    return source.slice(start).split('\n      - name: ')[0];
  };
  for (const name of ['Verify DEV access gateway with Nginx', 'Install Chromium system dependencies']) {
    const block = step(name);
    assert.match(block, /--attempts 1\b/, 'Privileged package setup must not overlap another apt/dpkg attempt');
    assert.match(block, /--timeout-seconds 300\b/, 'Cold system package setup needs its own bounded transaction');
    assert(Number(block.match(/timeout-minutes: (\d+)/)?.[1]) >= 6, 'Outer timeout must include transaction cleanup');
  }
  assert.match(step('Install Chromium system dependencies'), /playwright install-deps chromium\b/);
  const download = step('Install Chromium browser');
  assert.match(download, /--attempts 2\b/);
  assert.match(download, /--timeout-seconds 90\b/);
  assert.match(download, /playwright install chromium\b/);
  assert.doesNotMatch(download, /--with-deps|install-deps|apt-get|sudo/);
  assert(source.indexOf('name: Install Chromium system dependencies') < source.indexOf('name: Install Chromium browser'));
  assert(source.indexOf('name: Install Chromium browser') < source.indexOf('name: Verify session recovery across browser tabs'));
});
