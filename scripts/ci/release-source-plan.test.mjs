import assert from 'node:assert/strict';
import { mkdtempSync, mkdirSync, writeFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve, sep } from 'node:path';
import { spawnSync } from 'node:child_process';
import test from 'node:test';
import { createSourcePlan } from './release-source-plan.mjs';

const guards = ['web/platform/src/app/typography.contract.test.ts', 'web/platform/src/app/login/page.test.tsx'];
const commandsFor = (plan, cwd) => plan.commands.filter(command => command.cwd === cwd);
const assertFastWebChecks = (plan, pkg) => {
  const checks = commandsFor(plan, pkg);
  assert.equal(checks.some(c => c.command === 'npm' && c.args[0] === 'run' && c.args[1] === 'test'), false);
  assert.equal(checks.some(c => c.args.includes('related')), false);
  assert.deepEqual(checks.filter(c => c.command === 'npm').map(c => c.args.join(' ')),
    pkg === 'web/platform' ? ['run lint', 'run typecheck', 'run validate:assets'] : ['run lint', 'run typecheck']);
  const testChecks = checks.filter(c => c.command === 'node');
  assert.equal(testChecks.length, pkg === 'web/platform' ? 1 : 0);
  if (pkg === 'web/platform') {
    assert.deepEqual(testChecks[0].args.slice(0, 2), ['node_modules/vitest/vitest.mjs', 'run']);
    assert(testChecks[0].args.slice(2).length >= guards.length);
    assert(testChecks[0].args.slice(2).every(path => path === 'src/app/login/page.test.tsx'
      || /(?:[.-]contract|server-boundary)\.test\.tsx?$/.test(path)));
  }
};

test('page edits automatically select bounded lint, types, guards and assets and defer the full suite', () => {
  const file = 'web/platform/src/app/login/page.tsx';
  const plan = createSourcePlan([file], [...guards, file]);
  const checks = commandsFor(plan, 'web/platform');
  assert(checks.some(c => c.command === 'npm' && c.args.join(' ') === 'run lint'));
  assert(checks.some(c => c.command === 'npm' && c.args.join(' ') === 'run typecheck'));
  assert(checks.some(c => c.args.includes('src/app/typography.contract.test.ts') && c.args.includes('src/app/login/page.test.tsx')));
  assertFastWebChecks(plan, 'web/platform');
  assert.deepEqual(plan.deferredFullSuites, ['web/platform']);
  assert.equal(commandsFor(plan, 'web/admin').length, 0);
});

test('CSS edits still execute typography guards read through the filesystem', () => {
  const file = 'web/platform/src/features/public-home/PublicHome.module.css';
  const plan = createSourcePlan([file], [...guards, file]);
  assert(commandsFor(plan, 'web/platform').some(c => c.args.includes('src/app/typography.contract.test.ts')));
});

test('new tests and paths with spaces do not expand early tests', () => {
  const file = 'web/platform/src/features/new feature/New.test.tsx';
  const plan = createSourcePlan([file], [...guards, file]);
  assertFastWebChecks(plan, 'web/platform');
  assert.deepEqual(plan.deferredFullSuites, ['web/platform']);
});

test('missing mandatory regressions fail closed', () => {
  assert.throws(() => createSourcePlan(['web/platform/src/app/login/page.tsx'], []), /mandatory.*guard/i);
});

for (const pkg of ['web/platform', 'web/admin']) for (const relative of ['package-lock.json', 'vitest.config.ts', 'src/test/setup.ts', 'src/deleted.tsx']) {
  const file = `${pkg}/${relative}`;
  test(`broad or deleted frontend input keeps fast checks and defers the full suite: ${file}`, () => {
    const existing = file.includes('deleted') ? guards : [...guards, file];
    const plan = createSourcePlan([file], existing);
    assertFastWebChecks(plan, pkg);
    assert.deepEqual(plan.deferredFullSuites, [pkg]);
  });
}

test('admin edits select only the admin package', () => {
  const file = 'web/admin/src/App.tsx';
  const plan = createSourcePlan([file], [file]);
  assertFastWebChecks(plan, 'web/admin');
  assert.deepEqual(plan.deferredFullSuites, ['web/admin']);
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
  assert.deepEqual(plan.commands[0], { cwd: '.', command: 'node', args: ['scripts/ci/release-linux-check.mjs'] });
});

test('infrastructure admission precedes web checks and reports each affected package once', () => {
  const files = ['web/platform/src/page.tsx', 'web/admin/package-lock.json', 'docker-compose.dev.yml'];
  const plan = createSourcePlan([...files, files[0]], [...guards, ...files]);
  assert.deepEqual(plan.commands[0], { cwd: '.', command: 'node', args: ['scripts/ci/release-linux-check.mjs'] });
  assert.deepEqual(plan.deferredFullSuites, ['web/admin', 'web/platform']);
  for (const pkg of plan.deferredFullSuites) assertFastWebChecks(plan, pkg);
});

test('documentation edits do not trigger frontend or Go suites', () => {
  const files = ['docs/runbooks/DEV.md', 'web/platform/docs/ui-catalog.md'];
  const plan = createSourcePlan(files, files);
  assert.equal(commandsFor(plan, 'web/platform').length, 0);
  assert.equal(plan.commands.filter(c => c.command === 'go').length, 0);
  assert(plan.commands.some(c => c.args.includes('scripts/ci/validate-docs.ps1')));
  assert.deepEqual(plan.deferredFullSuites, []);
});

test('unsafe or option-like source paths are rejected', () => {
  for (const file of ['../secret', '/absolute', '-option', 'src/../secret', 'src\\file.ts', 'src/file\n.ts']) {
    assert.throws(() => createSourcePlan([file], []), /path/i);
  }
});

test('large frontend inventories cannot expand the early test command', () => {
  const files = Array.from({ length: 150 }, (_, i) => `web/platform/src/${'nested/'.repeat(20)}file-${i}.tsx`);
  const plan = createSourcePlan(files, [...guards, ...files]);
  assertFastWebChecks(plan, 'web/platform');
  assert.deepEqual(plan.deferredFullSuites, ['web/platform']);
  assert(plan.commands.every(c => c.args.join(' ').length < 12000));
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
