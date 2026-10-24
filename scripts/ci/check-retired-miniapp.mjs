import assert from 'node:assert/strict';
import { existsSync, readFileSync } from 'node:fs';

for (const path of ['web/miniapp', 'internal/app/miniapp', 'internal/adapter/inbound/miniapp', 'Dockerfile.miniapp', 'deployments/nginx/miniapp.static.conf', 'scripts/dev/start-miniapp.ps1', 'scripts/dev/_miniapp-common.ps1', 'start-miniapp-ngrok.ps1', 'observability/grafana/dashboards/miniapp-frontend.json', 'tests/k6/job-worker.js', 'tests/k6/billing-payments.js']) {
  assert(!existsSync(path), `Retired Mini App component remains: ${path}`);
}
for (const path of ['cmd/api/main.go', '.github/workflows/ci.yml', '.github/workflows/docker-images.yml', '.github/workflows/nightly-quality.yml', '.github/dependabot.yml', '.golangci.yml', 'docker-compose.prod.yml', 'scripts/ci/dev-deploy-preflight.ps1', 'deployments/nginx/nginx.prod.conf', 'observability/prometheus/prometheus.yml', 'web/admin/scripts/e2e-smoke.mjs', 'web/admin/playwright.config.ts', 'web/admin/e2e/admin.smoke.spec.ts']) {
  const source = readFileSync(path, 'utf8');
  assert(!/internal\/(?:app|adapter\/inbound)\/miniapp|web\/miniapp|miniapp\/node_modules|Dockerfile\.miniapp|^\s*miniapp:|http:\/\/miniapp:|Handle\("\/miniapp\//m.test(source), `Retired application wired in ${path}`);
}
const account = readFileSync('internal/adapter/inbound/account/handler.go', 'utf8');
assert(!/VerifyLaunchParams|X-VK-User-ID|X-Launch-Params/.test(account), 'Account API still uses retired launch authentication');
assert(account.includes('AuthenticateAccessToken'), 'Account API must verify existing server-issued sessions');
assert(!/VKAppID|VKAppSecret|MiniApp/.test(readFileSync('internal/platform/config/config.go', 'utf8')), 'Runtime configuration still exposes retired app settings');
assert(!/server_name[^;]*(?:\s|^)app\.epifera\.com|miniapp_frontend|location[^\n]*\/miniapp\//.test(readFileSync('deployments/nginx/nginx.prod.conf', 'utf8')), 'Retired proxy host or route remains');
console.log('VK Mini App retirement checks passed; no application, build, API mount or deployment service remains.');
