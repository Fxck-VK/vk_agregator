package domain

// User-safe media job error classes. These values may be exposed to clients,
// alerts and support tooling; keep them bounded and provider-agnostic.
const (
	JobErrMediaUploadInvalid         = "media_upload_invalid"
	JobErrMediaUploadTooLarge        = "media_upload_too_large"
	JobErrMediaUploadUnsupported     = "media_upload_unsupported"
	JobErrMediaProviderOutputInvalid = "media_provider_output_invalid"
	JobErrMediaProcessingUnavailable = "media_processing_unavailable"
	JobErrMediaDeliveryFailed        = "media_delivery_failed"
	JobErrMediaOverloadedRetryLater  = "media_overloaded_retry_later"
)
