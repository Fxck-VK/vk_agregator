import assert from 'node:assert/strict';
import { mkdtempSync, mkdirSync, writeFileSync, rmSync, copyFileSync, existsSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve, sep } from 'node:path';
import { spawnSync } from 'node:child_process';
import test from 'node:test';
import { createSourcePlan } from './release-source-plan.mjs';

const guards = ['web/platform/src/app/typography.contract.test.ts', 'web/platform/src/app/login/page.test.tsx'];
const commandsFor = (plan, cwd) => plan.commands.filter(command => command.cwd === cwd);

test('page edits automatically select lint, types, shared guards and related tests', () => {
  const file = 'web/platform/src/app/login/page.tsx';
  const plan = createSourcePlan([file], [...guards, file]);
  const checks = commandsFor(plan, 'web/platform');
  assert(checks.some(c => c.command === 'npm' && c.args.join(' ') === 'run lint'));
  assert(checks.some(c => c.command === 'npm' && c.args.join(' ') === 'run typecheck'));
  assert(checks.some(c => c.args.includes('src/app/typography.contract.test.ts') && c.args.includes('src/app/login/page.test.tsx')));
  assert(checks.some(c => c.args.includes('related') && c.args.includes('src/app/login/page.tsx') && c.args.includes('--run')));
  assert.equal(commandsFor(plan, 'web/admin').length, 0);
});

test('CSS edits still execute typography guards read through the filesystem', () => {
  const file = 'web/platform/src/features/public-home/PublicHome.module.css';
  const plan = createSourcePlan([file], [...guards, file]);
  assert(commandsFor(plan, 'web/platform').some(c => c.args.includes('src/app/typography.contract.test.ts')));
});

test('new tests and paths with spaces remain individual related-test arguments', () => {
  const file = 'web/platform/src/features/new feature/New.test.tsx';
  const plan = createSourcePlan([file], [...guards, file]);
  assert(commandsFor(plan, 'web/platform').some(c => c.args.includes('src/features/new feature/New.test.tsx')));
});

test('missing mandatory regressions fail closed', () => {
  assert.throws(() => createSourcePlan(['web/platform/src/app/login/page.tsx'], []), /mandatory.*guard/i);
});

for (const file of ['web/platform/package-lock.json', 'web/platform/vitest.config.ts', 'web/platform/src/test/setup.ts', 'web/platform/src/deleted.tsx']) {
  test(`broad or deleted frontend input falls back to the full package suite: ${file}`, () => {
    const existing = file.includes('deleted') ? guards : [...guards, file];
    const plan = createSourcePlan([file], existing);
    assert(commandsFor(plan, 'web/platform').some(c => c.command === 'npm' && c.args.join(' ') === 'run test'));
  });
}

test('admin edits select only the admin package', () => {
  const file = 'web/admin/src/App.tsx';
  const plan = createSourcePlan([file], [file]);
  assert(commandsFor(plan, 'web/admin').some(c => c.args.includes('related')));
  assert.equal(commandsFor(plan, 'web/platform').length, 0);
});

test('Go source selects formatting, affected package tests and vet', () => {
  const file = 'internal/service/example/service.go';
  const plan = createSourcePlan([file], [file]);
  assert(plan.commands.some(c => c.command === 'gofmt' && c.expectEmptyOutput && c.args.includes(file)));
  for (const action of ['test', 'vet']) assert(plan.commands.some(c => c.command === 'go' && c.args.join(' ') === `${action} ./internal/service/example`));
  const broad = createSourcePlan(['go.mod'], ['go.mod']);
  assert(broad.commands.some(c => c.command === 'go' && c.args.join(' ') === 'test ./...'));
});

test('release tooling changes select shared contract and orchestration regressions', () => {
  const plan = createSourcePlan(['scripts/ci/release-source-plan.mjs'], []);
  assert(plan.commands.some(c => c.args.includes('scripts/ci/release-source-check.test.mjs')));
  assert(plan.commands.some(c => c.args.includes('scripts/ci/release-contract.test.mjs')));
  assert(plan.commands.some(c => c.args.includes('scripts/ci/test-dev-deploy-preflight.ps1')));
});

test('documentation edits do not trigger frontend or Go suites', () => {
  const files = ['docs/runbooks/DEV.md', 'web/platform/docs/ui-catalog.md'];
  const plan = createSourcePlan(files, files);
  assert.equal(commandsFor(plan, 'web/platform').length, 0);
  assert.equal(plan.commands.filter(c => c.command === 'go').length, 0);
  assert(plan.commands.some(c => c.args.includes('scripts/ci/validate-docs.ps1')));
});

test('unsafe or option-like source paths are rejected', () => {
  for (const file of ['../secret', '/absolute', '-option', 'src/../secret', 'src\\file.ts', 'src/file\n.ts']) {
    assert.throws(() => createSourcePlan([file], []), /path/i);
  }
});

test('related argument batches stay bounded on Windows', () => {
  const files = Array.from({ length: 150 }, (_, i) => `web/platform/src/${'nested/'.repeat(20)}file-${i}.tsx`);
  const plan = createSourcePlan(files, [...guards, ...files]);
  const related = plan.commands.filter(c => c.args.includes('related'));
  assert(related.length > 1);
  assert(related.every(c => c.args.join(' ').length < 12000));
  assert.equal(related.flatMap(c => c.args.filter(p => p.endsWith('.tsx'))).length, files.length);
});

test('CLI includes committed, staged, unstaged, deleted and new paths against the base', t => {
  const root = mkdtempSync(join(tmpdir(), 'epifera-source-check-'));
  t.after(() => {
    assert(resolve(root).startsWith(resolve(tmpdir()) + sep));
    rmSync(root, { recursive: true, force: true });
  });
  const git = (...args) => {
    const result = spawnSync('git', args, { cwd: root, encoding: 'utf8' });
    assert.equal(result.status, 0, result.stderr); return result.stdout.trim();
  };
  git('init', '-q'); git('config', 'user.name', 'Source Fixture'); git('config', 'user.email', 'fixture@example.invalid');
  writeFileSync(join(root, '.gitignore'), '.cache/\n');
  for (const file of ['committed.txt', 'staged.txt', 'unstaged.txt', 'deleted.txt']) writeFileSync(join(root, file), 'before\n');
  git('add', '.'); git('commit', '-qm', 'baseline');
  const base = git('rev-parse', 'HEAD');
  writeFileSync(join(root, 'committed.txt'), 'after\n'); git('add', '.'); git('commit', '-qm', 'committed change');
  writeFileSync(join(root, 'staged.txt'), 'after\n'); git('add', 'staged.txt');
  writeFileSync(join(root, 'unstaged.txt'), 'after\n'); rmSync(join(root, 'deleted.txt'));
  writeFileSync(join(root, 'new file.txt'), 'new\n'); mkdirSync(join(root, '.cache')); writeFileSync(join(root, '.cache/private.txt'), 'ignored\n');
  const result = spawnSync(process.execPath, [resolve(import.meta.dirname, 'release-source-plan.mjs'), '--base', base], { cwd: root, encoding: 'utf8' });
  assert.equal(result.status, 0, result.stderr);
  assert.deepEqual(JSON.parse(result.stdout).changedFiles, ['committed.txt', 'deleted.txt', 'new file.txt', 'staged.txt', 'unstaged.txt']);
});

for (const autocrlf of ['false', 'true']) test(`CLI rejects whitespace errors before long checks with core.autocrlf=${autocrlf}`, t => {
  const root = mkdtempSync(join(tmpdir(), 'epifera-source-whitespace-'));
  t.after(() => {
    assert(resolve(root).startsWith(resolve(tmpdir()) + sep));
    rmSync(root, { recursive: true, force: true });
  });
  const git = (...args) => {
    const result = spawnSync('git', args, { cwd: root, encoding: 'utf8' });
    assert.equal(result.status, 0, result.stderr); return result.stdout.trim();
  };
  git('init', '-q'); git('config', 'user.name', 'Whitespace Fixture'); git('config', 'user.email', 'fixture@example.invalid');
  git('config', 'core.autocrlf', autocrlf);
  git('config', 'core.whitespace', 'blank-at-eol,blank-at-eof,space-before-tab');
  writeFileSync(join(root, '.gitignore'), '.cache/\n');
  git('add', '.'); git('commit', '-qm', 'base');
  const base = git('rev-parse', 'HEAD');
  const run = () => spawnSync(process.execPath, [resolve(import.meta.dirname, 'release-source-plan.mjs'), '--base', base], { cwd: root, encoding: 'utf8' });
  for (const content of ['new source\n\n', 'new source  \n']) {
    writeFileSync(join(root, 'new file.txt'), content);
    const result = run();
    assert.notEqual(result.status, 0);
    assert.match(result.stderr, /whitespace.*new file\.txt/i);
  }
  writeFileSync(join(root, 'new file.txt'), autocrlf === 'true' ? 'clean source\r\n' : 'clean source\n');
  writeFileSync(join(root, 'new.bin'), Buffer.from([0, 1, 10, 10]));
  mkdirSync(join(root, '.cache'));
  writeFileSync(join(root, '.cache/ignored.txt'), 'ignored  \n\n');
  const clean = run();
  assert.equal(clean.status, 0, clean.stderr);
  assert.deepEqual(JSON.parse(clean.stdout).changedFiles, ['new file.txt', 'new.bin']);
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
