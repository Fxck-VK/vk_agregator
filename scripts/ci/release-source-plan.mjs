import { spawnSync } from 'node:child_process';
import { existsSync, lstatSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { pathToFileURL } from 'node:url';

function safePath(path) {
  if (typeof path !== 'string' || !path || /^[/-]/.test(path) || /[\\\x00-\x1f]/.test(path)
    || path.split('/').some(part => ['..', '.', '.git'].includes(part))) throw new Error('Unsafe changed source path');
  return path;
}

export function createSourcePlan(changedFiles, existingFiles) {
  const changed = [...new Set(changedFiles.map(safePath))].sort();
  const existing = new Set(existingFiles);
  const commands = [];
  const deferredFullSuites = [];
  const add = (command, args, cwd = '.', extra = {}) => {
    const step = { cwd, command, args, ...extra };
    if (!commands.some(other => JSON.stringify(other) === JSON.stringify(step))) commands.push(step);
  };
  const infrastructureChanged = changed.some(path => /^(?:scripts\/(?:ci|deploy)\/|\.github\/workflows\/|Dockerfile\.|docker-compose|trivy-filesystem)/.test(path));
  if (infrastructureChanged) add('node', ['scripts/ci/release-linux-check.mjs']);

  for (const pkg of ['web/platform', 'web/admin']) {
    const inputs = changed.filter(path => path.startsWith(`${pkg}/`) && !path.startsWith(`${pkg}/docs/`) && !path.endsWith('.md'));
    if (!inputs.length) continue;
    deferredFullSuites.push(pkg);
    add('npm', ['run', 'lint'], pkg);
    add('npm', ['run', 'typecheck'], pkg);
    if (pkg === 'web/platform') {
      const required = ['src/app/typography.contract.test.ts', 'src/app/login/page.test.tsx'];
      for (const guard of required) {
        if (!existing.has(`${pkg}/${guard}`)) throw new Error(`Missing mandatory source guard: ${pkg}/${guard}`);
      }
      const guards = [...new Set([...required, ...existingFiles.filter(path => path.startsWith(`${pkg}/src/`)
        && /(?:[.-]contract|server-boundary)\.test\.tsx?$/.test(path)).map(path => path.slice(pkg.length + 1))])].sort();
      add('node', ['node_modules/vitest/vitest.mjs', 'run', ...guards], pkg);
      add('npm', ['run', 'validate:assets'], pkg);
    }
    // Full web suites belong to the final exact-commit preflight. Even narrow
    // related-test selection can expand to the full suite during preparation.
  }

  const goFiles = changed.filter(path => path.endsWith('.go'));
  const existingGo = goFiles.filter(path => existing.has(path));
  if (existingGo.length) add('gofmt', ['-l', ...existingGo], '.', { expectEmptyOutput: true });
  if (goFiles.length || changed.some(path => ['go.mod', 'go.sum'].includes(path))) {
    const broad = changed.some(path => ['go.mod', 'go.sum'].includes(path)) || goFiles.some(path => !existing.has(path));
    const packages = broad ? ['./...'] : [...new Set(goFiles.map(path => `./${dirname(path).replaceAll('\\', '/')}`))].sort();
    for (const action of ['test', 'vet']) add('go', [action, ...packages]);
  }

  if (infrastructureChanged) {
    const commonTests = ['scripts/ci/release-source-check.test.mjs', 'scripts/ci/release-contract.test.mjs', 'scripts/ci/release-manifest.test.mjs'];
    add('node', ['--test', ...commonTests]);
    add('node', ['scripts/ci/check-release-contract.mjs']);
    add('pwsh', ['-NoProfile', '-File', 'scripts/ci/test-dev-deploy-preflight.ps1']);
    for (const path of changed) {
      if (path.startsWith('scripts/') && path.endsWith('.mjs')) {
        const companion = path.endsWith('.test.mjs') ? path : path.replace(/\.mjs$/, '.test.mjs');
        if (existing.has(companion) && !commonTests.includes(companion)) add('node', ['--test', companion]);
      }
    }
  }
  if (changed.some(path => path.endsWith('.md') || path === '.agents/state.json')) {
    add('pwsh', ['-NoProfile', '-File', 'scripts/ci/validate-docs.ps1']);
  }
  return { changedFiles: changed, commands, deferredFullSuites: deferredFullSuites.sort() };
}

function git(...args) {
  const result = spawnSync('git', args, { encoding: 'utf8', maxBuffer: 32 * 1024 * 1024 });
  if (result.status !== 0) throw new Error('Cannot determine changed release source');
  return result.stdout;
}

function main() {
  const args = process.argv.slice(2);
  if (args.length !== 2 || args[0] !== '--base' || !args[1] || args[1].startsWith('-')) throw new Error('Use --base <DEV baseline>');
  const root = resolve(git('rev-parse', '--show-toplevel').trim());
  if (root !== resolve(process.cwd())) throw new Error('Run source checks from the repository root');
  const base = git('rev-parse', '--verify', `${args[1]}^{commit}`).trim();
  if (git('diff', '--name-only', '--diff-filter=U').trim()) throw new Error('Resolve unmerged source before preparing a release');
  const nul = value => value.split('\0').filter(Boolean);
  const untracked = nul(git('ls-files', '--others', '--exclude-standard', '-z'));
  const changed = [...nul(git('diff', '--name-only', '--no-renames', '-z', base, '--')), ...untracked];
  for (const path of changed) {
    safePath(path);
    if (existsSync(path) && !lstatSync(path).isFile()) throw new Error('Changed source must be a regular file');
  }
  // A baseline diff does not include untracked files. Use Git's own whitespace
  // rules for each new file before any lint/test commands can be selected.
  for (const path of untracked) {
    const result = spawnSync('git', ['diff', '--no-index', '--check', '--no-ext-diff', '--no-textconv', '--', '/dev/null', path], { encoding: 'utf8' });
    // --no-index also returns 1 for an ordinary clean added-file diff.
    if (![0, 1].includes(result.status)) throw new Error(`Whitespace check failed for new source: ${path}`);
  }
  const existing = nul(git('ls-files', '--cached', '--others', '--exclude-standard', '-z'))
    .filter(path => existsSync(path) && lstatSync(path).isFile());
  const plan = createSourcePlan(changed, existing);
  plan.base = base;
  plan.commands.unshift({ cwd: '.', command: 'git', args: ['diff', '--check', base, '--'] });
  console.log(JSON.stringify(plan));
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  try { main(); } catch (error) { console.error(error.message); process.exitCode = 1; }
}
