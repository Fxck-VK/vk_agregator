import { spawnSync } from 'node:child_process';
import { randomUUID } from 'node:crypto';
import { existsSync, lstatSync, readFileSync, readdirSync } from 'node:fs';
import { join, resolve } from 'node:path';
import { pathToFileURL } from 'node:url';

const tests = ['release-contract.test.mjs', 'release-manifest.test.mjs', 'release-source-plan.test.mjs', 'release-linux-check.test.mjs']
  .map(name => `scripts/ci/${name}`);

export function createLinuxCheckPlan({ root, platform = process.platform, containerName = `epifera-release-check-${randomUUID()}` }) {
  root = resolve(root);
  const args = ['--test', ...tests];
  if (platform === 'linux') return { root, timeoutMs: 180000, commands: [{ command: process.execPath, args }], cleanup: null };
  if (!/^epifera-release-check-[a-z0-9-]+$/.test(containerName)) throw Error('Invalid owned Linux check container name');
  const dockerfile = readFileSync(join(root, 'Dockerfile.platform'), 'utf8');
  const image = dockerfile.match(/^FROM (node:[0-9]+-alpine@sha256:[a-f0-9]{64}) AS dependencies\r?$/m)?.[1];
  if (!image) throw Error('Linux release checks require a pinned Node dependencies image');
  // Only explicit source/configuration files are mounted, never directories,
  // Git metadata, env files, credentials, outputs or the Docker socket.
  const paths = [...tests, 'scripts/ci/release-linux-check.mjs', 'scripts/ci/check-release-contract.mjs',
    'scripts/ci/release-manifest.mjs', 'scripts/ci/release-source-plan.mjs', 'scripts/ci/dev-deploy-preflight.ps1',
    'scripts/deploy/verify-release-images.sh', '.github/workflows/ci.yml', '.github/workflows/docker-images.yml',
    '.github/workflows/deploy-dev.yml', '.github/workflows/nightly-quality.yml', 'docker-compose.prod.yml', 'docker-compose.dev-web.yml',
    ...readdirSync(root).filter(name => /^Dockerfile\.[a-z0-9-]+$/.test(name))];
  const mounts = [];
  for (const path of paths) {
    const source = join(root, path);
    if (!existsSync(source) || !lstatSync(source).isFile() || lstatSync(source).isSymbolicLink()) throw Error(`Missing regular Linux check source: ${path}`);
    mounts.push('--mount', `type=bind,src=${source},dst=/src/${path},readonly`);
  }
  return { root, timeoutMs: 180000,
    commands: [{ command: 'docker', args: ['run', '--rm', '--init', '--name', containerName, '--security-opt', 'no-new-privileges',
      ...mounts, '--workdir', '/src', image, 'timeout', '-s', 'KILL', '150', 'sh', '-ec',
      `apk add --no-cache git >/dev/null; exec node ${args.join(' ')}`] }],
    cleanup: { command: 'docker', args: ['rm', '-f', containerName] } };
}

export function executeLinuxCheckPlan(plan, execute = spawnSync) {
  const output = [];
  try {
    for (const command of plan.commands) {
      const result = execute(command.command, command.args, { cwd: plan.root, encoding: 'utf8', timeout: plan.timeoutMs,
        killSignal: 'SIGKILL', maxBuffer: 16 * 1024 * 1024 });
      const text = `${result.stdout ?? ''}${result.stderr ?? ''}`;
      if (result.error || result.status !== 0) throw Error(`Linux release checks failed (${result.error?.code ?? result.status ?? 'spawn'}).\n${text}`);
      output.push(text);
    }
    return output;
  } catch (error) {
    if (plan.cleanup) {
      try { execute(plan.cleanup.command, plan.cleanup.args, { cwd: plan.root, encoding: 'utf8', timeout: 30000, maxBuffer: 1024 * 1024 }); }
      catch { /* Preserve the original failure; only this owned container is targeted. */ }
    }
    throw error;
  }
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  try {
    if (process.argv.length !== 2) throw Error('Linux release checks accept no bypass options');
    const root = resolve(import.meta.dirname, '../..');
    const plan = createLinuxCheckPlan({ root });
    for (const output of executeLinuxCheckPlan(plan)) process.stdout.write(output);
    console.log('Native Linux release contracts, inventory and source selection passed.');
  } catch (error) { console.error(error.message); process.exitCode = 1; }
}
