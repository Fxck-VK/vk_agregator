# Production Artifact Scanner Bypass

Status: accepted

## Goal

Allow production deployment without an OpenAI credential while keeping the
default production configuration fail-closed.

## Configuration Contract

The only supported unscanned production configuration is:

```env
ARTIFACT_SCANNER=none
ALLOW_UNSCANNED_ARTIFACTS_IN_PRODUCTION=true
```

`OPENAI_API_KEY` is not required for that configuration. Production still
rejects `ARTIFACT_SCANNER=none` when the explicit bypass is false or absent.
When `ARTIFACT_SCANNER=openai`, a non-empty `OPENAI_API_KEY` remains required.

## Runtime Behavior

This change only affects configuration validation and deployment preflight.
Provider routing, moderation order, billing reservation/capture, artifact
ownership, storage, and delivery behavior remain unchanged. Generated
artifacts are delivered without malware scanning while the explicit bypass is
enabled.

## Deployment

The production secret fragment must set the two variables above and remove
`OPENAI_API_KEY`. The assembled environment must contain each key once.

## Tests

- Production with scanner disabled and no bypass is rejected.
- Production with scanner disabled and explicit bypass is accepted.
- OpenAI scanner without an OpenAI key is rejected.
- Deploy preflight enforces the same contract as runtime validation.
- Existing production config and deploy tests remain green.

## Security Impact

The bypass is an explicit risk acceptance, not a default. It removes malware
scanning from generated artifacts. Re-enabling scanning requires setting
`ARTIFACT_SCANNER=openai`, adding `OPENAI_API_KEY`, and setting the bypass to
false.
