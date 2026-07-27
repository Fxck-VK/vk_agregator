# NeiroHub Web Platform

This directory is the reserved frontend boundary for the standalone NeiroHub
web application. It is intentionally not runnable yet: framework setup and
dependencies will be added together with the first approved web feature.

## Architecture Boundary

The web platform is an independent deployable frontend inside the existing
monorepo. It shares the backend account, billing, job, artifact, and provider
routing services with the VK Bot and VK Mini App, but it does not share their
channel-specific UI code.

The platform must:

- authenticate through the shared Account Layer and web account sessions;
- call backend-owned HTTP APIs for jobs, payments, balances, and artifacts;
- treat all client state as untrusted and display backend-calculated values;
- use owner-checked artifact endpoints or short-lived signed URLs;
- build and deploy independently from `web/miniapp` and `web/admin`.

The platform must not:

- access Postgres, Redis, S3, AI providers, or payment providers directly;
- use VK launch parameters as its general web authentication contract;
- depend on `/miniapp/*` as its long-term channel-neutral API;
- contain backend secrets or mutate trusted billing/job state locally.

## Repository Layout

```text
web/
  miniapp/   VK Mini App
  admin/     Operator application
  platform/  Standalone web application
```

When implementation starts, this directory should receive its own package
manifest, source tree, tests, build configuration, Docker image, and CI job.
Those files should be introduced as one tested vertical slice rather than as
an empty framework scaffold.
