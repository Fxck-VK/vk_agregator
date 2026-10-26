import assert from 'node:assert/strict';
import { existsSync, lstatSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve, sep } from 'node:path';
import test from 'node:test';

const root = resolve(import.meta.dirname, '../..');
async function checker() {
  assert(existsSync(join(import.meta.dirname, 'release-linux-check.mjs')), 'Native Linux release guard is missing');
  return import('./release-linux-check.mjs');
}

test('Linux checks run complete portable contract, inventory and selector files without name filters', async () => {
  const { createLinuxCheckPlan } = await checker();
  const plan = createLinuxCheckPlan({ root, platform: 'linux' });
  assert.equal(plan.commands.length, 1);
  assert.equal(plan.commands[0].command, process.execPath);
  for (const name of ['release-contract.test.mjs', 'release-manifest.test.mjs', 'release-source-plan.test.mjs', 'release-linux-check.test.mjs']) {
    assert(plan.commands[0].args.includes(`scripts/ci/${name}`));
  }
  assert(!plan.commands[0].args.some(arg => arg.includes('test-name-pattern')));
  assert.equal(plan.cleanup, null);
});

test('Windows uses pinned source-only read-only mounts, a bounded container and no secrets', async () => {
  const { createLinuxCheckPlan } = await checker();
  const plan = createLinuxCheckPlan({ root, platform: 'win32', containerName: 'epifera-release-check-fixture' });
  const command = plan.commands[0];
  assert.equal(command.command, 'docker');
  assert(command.args.includes('--rm'));
  assert(command.args.includes('--init'), 'A separate init process must allow the workload deadline to signal its child');
  assert(command.args.some(arg => /^node:24-alpine@sha256:[a-f0-9]{64}$/.test(arg)));
  const imageIndex = command.args.findIndex(arg => /^node:24-alpine@sha256:/.test(arg));
  assert.deepEqual(command.args.slice(imageIndex + 1, imageIndex + 6), ['timeout', '-s', 'KILL', '150', 'sh'],
    'The container workload must remain bounded if its host process is cancelled');
  const mounts = command.args.filter(arg => arg.startsWith('type=bind,'));
  assert(mounts.length > 5);
  assert(mounts.every(arg => arg.endsWith(',readonly')));
  assert(mounts.every(arg => lstatSync(arg.split('src=')[1].split(',dst=')[0]).isFile()), 'Only explicit source files may be mounted');
  assert(!mounts.some(arg => /(?:^|[/\\])(?:\.git|\.env|\.agents)(?:[/\\,]|$)/.test(arg)));
  assert(!mounts.some(arg => arg.includes('docker.sock')));
  assert.equal(plan.timeoutMs, 180000);
  assert.deepEqual(plan.cleanup.args, ['rm', '-f', 'epifera-release-check-fixture']);
});

test('unpinned Linux images fail before any container can run', async t => {
  const { createLinuxCheckPlan } = await checker();
  const directory = mkdtempSync(join(tmpdir(), 'epifera-linux-pin-'));
  t.after(() => {
    assert(resolve(directory).startsWith(resolve(tmpdir()) + sep));
    rmSync(directory, { recursive: true, force: true });
  });
  writeFileSync(join(directory, 'Dockerfile.platform'), 'FROM node:24-alpine AS dependencies\n');
  assert.throws(() => createLinuxCheckPlan({ root: directory, platform: 'win32' }), /pinned/i);
});

test('failed and timed-out checks propagate failure and clean only their owned container', async () => {
  const { createLinuxCheckPlan, executeLinuxCheckPlan } = await checker();
  const plan = createLinuxCheckPlan({ root, platform: 'win32', containerName: 'epifera-release-check-owned' });
  for (const result of [{ status: 23, stdout: '', stderr: 'fixture failure' }, { status: null, error: new Error('ETIMEDOUT') }]) {
    const calls = [];
    assert.throws(() => executeLinuxCheckPlan(plan, (command, args) => {
      calls.push({ command, args });
      return calls.length === 1 ? result : { status: 0 };
    }), /Linux release checks failed/i);
    assert.equal(calls.length, 2);
    assert.deepEqual(calls[1], { command: 'docker', args: ['rm', '-f', 'epifera-release-check-owned'] });
  }
});

test('successful checks do not retry commands or create a false cleanup failure', async () => {
  const { createLinuxCheckPlan, executeLinuxCheckPlan } = await checker();
  const plan = createLinuxCheckPlan({ root, platform: 'linux' });
  let calls = 0;
  const output = executeLinuxCheckPlan(plan, () => { calls += 1; return { status: 0, stdout: 'passed', stderr: '' }; });
  assert.equal(calls, 1);
  assert.deepEqual(output, ['passed']);
});
