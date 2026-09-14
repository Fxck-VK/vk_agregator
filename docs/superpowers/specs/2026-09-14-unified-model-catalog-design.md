# Unified web model catalog

Status: implementation of the catalog design already approved in the conversation.

## Outcome

One authenticated `GET /web/v1/models` response supplies every functional model
consumer in the standalone web app. A model has one stable public ID, name,
description, category memberships, primary purpose and typed operations. The
header opens a new conversation; the conversation selector changes the current
conversation. Presentation and navigation do not own model facts.

## Ownership

Extend `internal/service/productcatalog`, keeping `providermodels` as the private
routing registry and `pricingcatalog` as the price authority. Reuse current
readiness filtering and generation resolvers. Public responses must omit provider
codes, endpoints, credentials, source reports and internal price calculations.
The response contains only models usable through the current web surface. Models
requiring unimplemented inputs remain unavailable; this migration does not enable
new provider routes or media-upload modes.

The six category IDs are `popular`, `images`, `text`, `video-audio`, `free`,
`study-work`. Backend membership is explicit output, not guessed independently by
each screen. Existing popularity may include all available models; picker
curation is ordered IDs only and is limited to five per category. Full catalog
shows all matches. No fake upcoming models enter functional model data.

## Contract

Envelope: `schema_version: 1`, `default_model_id`, `items`.
Each item: `id`, `name`, `description`, `kind` (text/image/video/audio),
`categories`, `verification` (legacy-unverified/verified-contract), optional
public `version`, and `operations`.
Each operation: `id`, `kind`, `enabled`, `inputs` (the safe input capability shape
from modelcontract), and exactly one of `text`, `image`, `video`, `audio`.
Text controls include server price, max prompt bytes and max output tokens.
Image controls include allowed quality, aspect ratios, defaults, maximum results,
reference limits and exact server prices per quality/aspect-ratio variant.
Video controls include valid resolution/duration/aspect combinations, defaults,
server prices per option, and supported input/output modes. Output combinations
must obey the runtime route and price snapshot. Audio has a typed contract but
no invented available model.

Legacy capabilities describe enforced runtime behavior and retain an explicit
unverified status. Unknown provider input support remains unknown; the web input
`enabled` state remains false where owned uploads are not implemented. Existing
contracts and reports are projected only after admission succeeds. No paid tests
are authorized by this refactor; actual provider re-verification requires a
concrete test matrix and a separately authorized provider/budget.

## Frontend

One strict parser and one 60-second in-memory request cache load the envelope.
Image/chat/video compatibility loaders become projections of this cache. Header,
composer, new chat, full catalog, home, image generator and file tools therefore
share data and failures. Model card descriptions come from the response.
Filters use category memberships. Controls use server options/defaults and valid
combinations. Marketing examples may remain curated content; they must not feed
functional selection or advertise invented available routes. Local preview uses
a fixture generated from the same backend builder, not a second handwritten list.

## Validation

Behavior tests cover unified membership, unique IDs, safe serialization, default
membership, unavailable models, priced options and current generation validation.
Frontend tests cover one request across consumer types, common descriptions,
category reordering, five-item picker limits, unlimited full catalog, options
compatibility and failure handling. Existing auth, jobs, ownership, billing,
idempotency and provider-only-in-workers boundaries remain mandatory.

No commit, push or deployment is part of this task.
