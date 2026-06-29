# Incident Runbook

Use this when production or DEV behavior is broken.

## First Five Minutes

1. Identify environment: production, DEV, local or loadtest.
2. Do not paste secrets, tokens, raw prompts, provider payloads, private URLs or
   PII into chat/logs.
3. Check latest deploy and image tag.
4. Run environment-specific smoke.
5. Check logs only with sanitized excerpts.

Production smoke:

```bash
bash scripts/deploy/smoke-prod.sh --env-file .env
```

DEV smoke:

```powershell
.\scripts\dev\smoke-dev.ps1
```

## Common Failure Areas

| Symptom | First checks |
| --- | --- |
| VK bot does not answer | VK Callback URL, tunnel, `cmd/api`, VK secret/confirmation, rate limit |
| Mini App does not open | `app` domain route, static frontend, `/miniapp/*` BFF, launch signature |
| YooKassa webhook missing | public route, `cmd/provider-webhook`, HTTPS, event selection |
| Jobs stuck | Redis stream depth, worker health, provider task state, DLQ |
| Provider unavailable | provider health metrics, circuit breaker state, credentials, quota |
| Balance wrong | ledger entries, payment intent state, reservation/capture/release |
| Admin exposed | reverse proxy route, Cloudflare Access/VPN, smoke blocked URLs |
| Deploy failed | GitHub Actions summary, image tag, smoke failure, rollback result |

## Logs

Use aggregate errors and classes where possible. Safe examples:

```text
provider=poyo class=rate_limited count=12
queue=jobs depth=240 oldest_age=180s
payment_webhook_unprocessed=3 oldest_age=90s
```

Unsafe examples:

- full VK launch params;
- auth headers;
- payment provider raw payloads;
- customer email/phone;
- prompts;
- private artifact URLs;
- API keys/tokens.

## Admin/Operator Incidents

Admin/operator routes must be protected. If public smoke or blackbox reports
`2xx` on `/admin/*`:

1. treat it as security incident;
2. close route at reverse proxy/Cloudflare immediately;
3. rotate exposed tokens if any response leaked credentials;
4. inspect audit log for suspicious operator actions;
5. re-run production smoke.

## Repeated/Non-Obvious Failures

If the root cause and fix are reusable, append a sanitized entry to
`.agents/logs/errors.jsonl`. Do not log secrets or PII.
