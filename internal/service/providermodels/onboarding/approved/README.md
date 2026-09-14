# Approved model contracts

Only ready, reviewed JSON contracts belong here. Drafts belong with their task's
verification report and are validated using `go run ./scripts/models/check
-contract <path>`. No example is pre-approved, and no live result may be invented.

See [model onboarding](../../../../../docs/runbooks/MODEL_ONBOARDING.md).

Files are embedded in the application. Changing this directory requires rebuilding
the application. Public APIs must never expose evidence, provider IDs or endpoints.
