import { test } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtempSync, mkdirSync, writeFileSync, readFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const script = fileURLToPath(new URL('./release-manifest.mjs', import.meta.url));
function fixture(t) {
  const root = mkdtempSync(join(tmpdir(), 'epifera-release-'));
  t.after(() => { assert.ok(resolve(root).startsWith(resolve(tmpdir()))); rmSync(root, { recursive: true, force: true }); });
  const git = (...args) => {
    const r = spawnSync('git', args, { cwd: root, encoding: 'utf8' });
    assert.equal(r.status, 0, r.stderr); return r.stdout.trim();
  };
  const run = (...args) => spawnSync(process.execPath, [script, ...args], { cwd: root, encoding: 'utf8' });
  git('init', '-q'); git('config', 'user.name', 'Release Fixture'); git('config', 'user.email', 'fixture@example.invalid');
  writeFileSync(join(root, '.gitignore'), '.cache/\n');
  writeFileSync(join(root, 'theme.txt'), 'old theme\n');
  writeFileSync(join(root, 'legal.txt'), 'old documents\n');
  git('add', '.'); git('commit', '-qm', 'fixture baseline');
  mkdirSync(join(root, '.cache'));
  const manifest = '.cache/release.json';
  return { root, git, run, manifest, write: (p, text) => writeFileSync(join(root, p), text) };
}
test('forgotten change blocks publication even when another planned change was committed', t => {
  const f = fixture(t); f.write('theme.txt', 'new theme\n'); f.write('legal.txt', 'new documents\n');
  assert.equal(f.run('--capture', f.manifest, '--base', 'HEAD').status, 0);
  f.git('add', 'legal.txt'); f.git('commit', '-qm', 'incomplete release');
  const r = f.run('--check', f.manifest, '--commit', 'HEAD');
  assert.notEqual(r.status, 0); assert.match(r.stderr, /theme.txt/);
});
test('complete planned edit, new file and deletion pass; CRLF conversion follows Git blobs', t => {
  const f = fixture(t); f.git('config', 'core.autocrlf', 'true');
  f.write('theme.txt', 'new theme\r\n'); f.write('new file.txt', 'new\r\n'); rmSync(join(f.root, 'legal.txt'));
  assert.equal(f.run('--capture', f.manifest, '--base', 'HEAD').status, 0);
  f.git('add', '.'); f.git('commit', '-qm', 'complete release');
  const r = f.run('--check', f.manifest, '--commit', 'HEAD'); assert.equal(r.status, 0, r.stderr);
});
test('unexpected and modified-after-capture files cannot silently enter the release', t => {
  const f = fixture(t); f.write('theme.txt', 'approved\n');
  assert.equal(f.run('--capture', f.manifest, '--base', 'HEAD').status, 0);
  f.write('theme.txt', 'different\n'); f.write('extra.txt', 'unplanned\n'); f.git('add', '.'); f.git('commit', '-qm', 'drift');
  const r = f.run('--check', f.manifest, '--commit', 'HEAD'); assert.notEqual(r.status, 0); assert.match(r.stderr, /theme.txt|extra.txt/);
});
test('deferral requires an explicit reason and cannot include the deferred edit', t => {
  const f = fixture(t); f.write('theme.txt', 'future\n'); f.write('legal.txt', 'approved\n');
  assert.equal(f.run('--capture', f.manifest, '--base', 'HEAD').status, 0);
  const p = join(f.root, f.manifest), m = JSON.parse(readFileSync(p));
  const item = m.files.find(v => v.path === 'theme.txt'); item.decision = 'defer';
  writeFileSync(p, JSON.stringify(m)); f.git('add', 'legal.txt'); f.git('commit', '-qm', 'scoped release');
  assert.notEqual(f.run('--check', f.manifest).status, 0);
  item.reason = 'Explicitly outside this release'; writeFileSync(p, JSON.stringify(m));
  assert.equal(f.run('--check', f.manifest).status, 0);
  f.git('add', 'theme.txt'); f.git('commit', '-qm', 'unapproved addition');
  assert.notEqual(f.run('--check', f.manifest).status, 0);
});
test('capture refuses to overwrite an existing release inventory', t => {
  const f = fixture(t); f.write('theme.txt', 'approved\n');
  assert.equal(f.run('--capture', f.manifest, '--base', 'HEAD').status, 0);
  assert.notEqual(f.run('--capture', f.manifest, '--base', 'HEAD').status, 0);
});
test('an approved executable-bit change uses the Git index on Windows', t => {
  const f = fixture(t); f.git('config', 'core.filemode', 'false');
  f.git('update-index', '--chmod=+x', 'theme.txt');
  assert.equal(f.run('--capture', f.manifest, '--base', 'HEAD').status, 0);
  f.git('commit', '-qm', 'approved file mode');
  const r = f.run('--check', f.manifest); assert.equal(r.status, 0, r.stderr);
});
