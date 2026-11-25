package websession

import (
	"testing"

	"vk-ai-aggregator/internal/domain"
)

func TestPresentationFailureDistinguishesUnconfirmedSubmitAndOutputStorage(t *testing.T) {
	for _, scenario := range []struct {
		class domain.ProviderErrorClass
		code  string
	}{
		{domain.ProviderErrSubmitIndeterminate, "submission_unconfirmed"},
		{domain.ProviderErrOutputDownloadFailed, "result_storage_failed"},
	} {
		t.Run(scenario.code, func(t *testing.T) {
			job := &domain.Job{
				Status:       domain.JobStatusFailedTerminal,
				ErrorCode:    string(scenario.class),
				ErrorMessage: "raw provider diagnostic must remain private",
			}
			if got := presentationFailureCode(job); got != scenario.code {
				t.Fatalf("presentation failure code = %q, want %q", got, scenario.code)
			}
			job.Status = domain.JobStatusProviderSucceeded
			if got := presentationFailureCode(job); got != "" {
				t.Fatalf("intermediate status exposed terminal error: %q", got)
			}
		})
	}
}
