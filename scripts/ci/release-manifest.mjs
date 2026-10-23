import { spawnSync } from 'node:child_process';
import { readFileSync, writeFileSync, mkdirSync, existsSync, lstatSync } from 'node:fs';
import { dirname, resolve } from 'node:path';

function git(...args) {
  const r = spawnSync('git', args, { encoding: 'utf8', maxBuffer: 32 * 1024 * 1024 });
  if (r.status !== 0) throw new Error('Git inventory operation failed');
  return r.stdout;
}
const nul = s => s.split('\0').filter(Boolean);
const validPath = p => typeof p === 'string' && p.length > 0 && !p.startsWith('/') && !p.split('/').some(v => v === '..' || v === '.git') && !p.includes('\\');
function tree(commit) {
  return new Map(nul(git('ls-tree', '-r', '-z', commit)).map(row => {
    const split = row.indexOf('\t'), metadata = row.slice(0, split), path = row.slice(split + 1);
    const [mode, type, blob] = metadata.split(' ');
    if (type !== 'blob') throw new Error('Submodules are not supported by the release inventory');
    return [path, { mode, blob }];
  }));
}
function main() {
  const args = process.argv.slice(2), options = {};
  for (let i = 0; i < args.length; i += 2) {
    if (!['--capture', '--check', '--base', '--commit'].includes(args[i]) || !args[i + 1] || options[args[i]]) throw new Error('Invalid release inventory arguments');
    options[args[i]] = args[i + 1];
  }
  if (!!options['--capture'] === !!options['--check']) throw new Error('Choose --capture or --check');
  const root = resolve(git('rev-parse', '--show-toplevel').trim());
  if (root !== resolve(process.cwd())) throw new Error('Run the inventory from the repository root');
  if (options['--capture']) {
    if (!options['--base']) throw new Error('--base is required when capturing a release');
    if (options['--base'].startsWith('-')) throw new Error('Invalid base revision');
    const path = resolve(options['--capture']);
    if (existsSync(path)) throw new Error('Release inventory already exists; review it instead of overwriting');
    const base = git('rev-parse', '--verify', `${options['--base']}^{commit}`).trim();
    const previous = tree(base);
    const index = new Map(nul(git('ls-files', '--stage', '-z')).map(row => {
      const split = row.indexOf('\t');
      return [row.slice(split + 1), row.slice(0, split).split(' ')[0]];
    }));
    const fileModes = git('config', '--bool', 'core.filemode').trim() === 'true';
    const changed = new Set([...nul(git('diff', '--name-only', '--no-renames', '-z', base)), ...nul(git('ls-files', '--others', '--exclude-standard', '-z'))]);
    const files = [...changed].sort().map(p => {
      if (!validPath(p)) throw new Error('Unsafe inventory path');
      const disk = resolve(root, p);
      if (!existsSync(disk)) return { path: p, blob: null, mode: null, decision: 'include' };
      const stat = lstatSync(disk);
      if (!stat.isFile()) throw new Error(`Release inventory requires regular files: ${p}`);
      const blob = git('hash-object', `--path=${p}`, '--', p).trim();
      const mode = fileModes ? (stat.mode & 0o111 ? '100755' : '100644') : (index.get(p) ?? previous.get(p)?.mode ?? '100644');
      return { path: p, blob, mode, decision: 'include' };
    });
    if (!files.length) throw new Error('No release changes to capture');
    mkdirSync(dirname(path), { recursive: true });
    writeFileSync(path, JSON.stringify({ schema: 1, base, files }, null, 2) + '\n', { flag: 'wx' });
    console.log(`Captured ${files.length} planned file changes; review include/defer decisions before committing.`);
    return;
  }
  const manifest = JSON.parse(readFileSync(options['--check'], 'utf8'));
  if (manifest.schema !== 1 || !/^[0-9a-f]{40}$/.test(manifest.base) || !Array.isArray(manifest.files) || !manifest.files.length) throw new Error('Invalid release inventory');
  const commit = git('rev-parse', '--verify', `${options['--commit'] ?? 'HEAD'}^{commit}`).trim();
  git('merge-base', '--is-ancestor', manifest.base, commit);
  const before = tree(manifest.base), after = tree(commit), planned = new Set(), seen = new Set();
  for (const f of manifest.files) {
    if (!validPath(f.path) || seen.has(f.path)) throw new Error('Invalid or duplicate inventory path');
    seen.add(f.path);
    if (f.decision === 'defer') {
      if (typeof f.reason !== 'string' || !f.reason.trim()) throw new Error(`Deferral needs a reason: ${f.path}`);
      if (JSON.stringify(before.get(f.path)) !== JSON.stringify(after.get(f.path))) throw new Error(`Deferred file entered release: ${f.path}`);
    } else if (f.decision === 'include') {
      planned.add(f.path);
      if (f.blob === null) {
        if (after.has(f.path)) throw new Error(`Planned deletion missing: ${f.path}`);
      } else if (!/^[0-9a-f]{40}$/.test(f.blob) || after.get(f.path)?.blob !== f.blob || after.get(f.path)?.mode !== f.mode) {
        throw new Error(`Planned change missing or modified: ${f.path}`);
      }
    } else throw new Error(`Choose include or defer: ${f.path}`);
  }
  if (!planned.size) throw new Error('Release contains no included changes');
  for (const path of nul(git('diff', '--name-only', '--no-renames', '-z', manifest.base, commit))) {
    if (!planned.has(path)) throw new Error(`Unexpected release change: ${path}`);
  }
  console.log(`Release inventory verified for ${commit}: ${planned.size} included, ${seen.size - planned.size} explicitly deferred.`);
}
try { main(); } catch (e) { console.error(e.message); process.exitCode = 1; }
