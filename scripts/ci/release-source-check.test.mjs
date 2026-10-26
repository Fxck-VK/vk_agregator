import assert from 'node:assert/strict';
import { mkdtempSync, mkdirSync, writeFileSync, rmSync, copyFileSync, existsSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve, sep } from 'node:path';
import { spawnSync } from 'node:child_process';
import test from 'node:test';
import './release-source-plan.test.mjs';

test('nightly infrastructure policy accepts the current release preflight', () => {
  const result = spawnSync('pwsh', ['-NoProfile', '-File', 'scripts/ci/validate-nightly-quality.ps1'], {
    cwd: resolve(import.meta.dirname, '../..'), encoding: 'utf8', timeout: 30000,
  });
  assert.equal(result.status, 0, `${result.stdout ?? ''}${result.stderr ?? ''}`);
});

function preparationFixture(t, checker) {
  const root = mkdtempSync(join(tmpdir(), 'epifera-preparation-'));
  t.after(() => {
    assert(resolve(root).startsWith(resolve(tmpdir()) + sep));
    rmSync(root, { recursive: true, force: true });
  });
  const scripts = join(root, 'scripts/ci');
  mkdirSync(scripts, { recursive: true });
  mkdirSync(join(root, '.cache'));
  for (const file of ['dev-deploy-preflight.ps1', 'DevDeployPreflight.psm1', 'TrivyFilesystemScope.psm1', 'BashRuntime.psm1']) {
    copyFileSync(resolve(import.meta.dirname, file), join(scripts, file));
  }
  writeFileSync(join(scripts, 'release-source-check.ps1'), checker);
  writeFileSync(join(scripts, 'release-manifest.mjs'), "import { writeFileSync } from 'node:fs'; writeFileSync('.cache/captured.json', '{}');");
  const run = () => spawnSync('pwsh', ['-NoProfile', '-File', join(scripts, 'dev-deploy-preflight.ps1'), '-Prepare', '-BaseRef', 'HEAD', '-ManifestPath', '.cache/captured.json'], { cwd: root, encoding: 'utf8' });
  return { root, scripts, run };
}

test('preparation failure stops before inventory capture', t => {
  const f = preparationFixture(t, 'param([string]$BaseRef)\nexit 23\n');
  const result = f.run();
  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /Early source checks failed/);
  assert.equal(existsSync(join(f.root, '.cache/captured.json')), false);
});

test('successful preparation captures only after the gate and refuses an existing inventory', t => {
  const f = preparationFixture(t, "param([string]$BaseRef)\n[IO.File]::WriteAllText('.cache/checked', 'passed')\nexit 0\n");
  const result = f.run();
  assert.equal(result.status, 0, result.stderr);
  assert(existsSync(join(f.root, '.cache/checked')));
  assert(existsSync(join(f.root, '.cache/captured.json')));
  rmSync(join(f.root, '.cache/checked'));
  const duplicate = f.run();
  assert.notEqual(duplicate.status, 0);
  assert.match(duplicate.stderr, /already exists/);
  assert.equal(existsSync(join(f.root, '.cache/checked')), false);
});

test('the actual source runner stops on a native command failure', t => {
  const f = preparationFixture(t, '');
  copyFileSync(resolve(import.meta.dirname, 'release-source-check.ps1'), join(f.scripts, 'release-source-check.ps1'));
  writeFileSync(join(f.scripts, 'release-source-plan.mjs'), `console.log(JSON.stringify({ changedFiles: ['fixture.txt'], commands: [
    {cwd: '.', command: 'node', args: ['-e', 'process.exit(23)']},
    {cwd: '.', command: 'node', args: ['-e', "require('node:fs').writeFileSync('.cache/later-check', '')"]}
  ] }));`);
  const result = f.run();
  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /Early source check failed/);
  assert.equal(existsSync(join(f.root, '.cache/later-check')), false);
  assert.equal(existsSync(join(f.root, '.cache/captured.json')), false);
});
