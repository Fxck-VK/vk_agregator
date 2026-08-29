# Local development port 7158

## Goal

Make the platform frontend use port `7158` by default during local development so it does not conflict with other local services.

## Design

Change the `dev` script in `web/platform/package.json` from `next dev` to `next dev --port 7158`. Developers will continue to use the existing `npm run dev` command, and Next.js will serve the application at `http://localhost:7158`.

The explicit Next.js CLI flag is preferred over a `PORT` environment variable because it is cross-platform, requires no new dependency, and keeps the configuration visible in the existing script.

## Scope

- Change only the local frontend development command.
- Do not change the production `start` command.
- Do not change Docker, CI/CD, reverse-proxy, API, or deployment ports.
- Do not add dependencies or environment variables.

## Verification

- Add a packaging assertion that the `dev` script contains the fixed port.
- Run the assertion before the change and observe the expected failure.
- Update the script and observe the assertion pass.
- Start the development server through `npm run dev` and confirm that Next.js listens on `http://localhost:7158`.
