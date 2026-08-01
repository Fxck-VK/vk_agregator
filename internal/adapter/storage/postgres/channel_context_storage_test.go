package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

func TestChannelNeutralRepositoryBindings(t *testing.T) {
	ctx := context.Background()
	accountID := uuid.New()

	t.Run("job persists neutral context and account-history mode", func(t *testing.T) {
		recorder := &nullableRecorder{}
		err := NewJobRepository(recorder).Create(ctx, &domain.Job{
			AccountID: accountID,
			ChannelContext: &domain.ChannelContext{
				Channel:      domain.ChannelWeb,
				RecipientRef: "browser-session",
				ThreadRef:    "thread-1",
			},
			ResultMode: domain.ResultModeAccountHistory,
		})
		assertNullableRecorderError(t, err)
		if len(recorder.args) < 30 {
			t.Fatalf("job binding argument count = %d, want neutral fields included", len(recorder.args))
		}
		assertStringArgument(t, recorder.args, 4, "web", "job channel")
		assertStringArgument(t, recorder.args, 5, "browser-session", "job recipient ref")
		assertStringArgument(t, recorder.args, 6, "thread-1", "job thread ref")
		assertStringArgument(t, recorder.args, 7, string(domain.ResultModeAccountHistory), "job result mode")
		assertNilArgument(t, recorder.args, 8, "job target channel")
		assertNilArgument(t, recorder.args, 9, "job target recipient ref")
		assertNilArgument(t, recorder.args, 10, "job target thread ref")
	})

	t.Run("delivery persists account owner and explicit target", func(t *testing.T) {
		recorder := &nullableRecorder{}
		err := NewDeliveryRepository(recorder).Create(ctx, &domain.Delivery{
			JobID:     uuid.New(),
			AccountID: accountID,
			Target: &domain.DeliveryTarget{
				Channel:      domain.ChannelVKBot,
				RecipientRef: "peer:2000000001",
				ThreadRef:    "thread-1",
			},
		})
		assertNullableRecorderError(t, err)
		if len(recorder.args) < 19 {
			t.Fatalf("delivery binding argument count = %d, want neutral fields included", len(recorder.args))
		}
		if got, ok := recorder.args[3].(*uuid.UUID); !ok || got == nil || *got != accountID {
			t.Fatalf("delivery account owner argument = %#v, want %s", recorder.args[3], accountID)
		}
		assertStringArgument(t, recorder.args, 6, "vk_bot", "delivery target channel")
		assertStringArgument(t, recorder.args, 7, "peer:2000000001", "delivery target recipient ref")
		assertStringArgument(t, recorder.args, 8, "thread-1", "delivery target thread ref")
		assertNilArgument(t, recorder.args, 4, "delivery legacy vk peer")
		assertNilArgument(t, recorder.args, 11, "delivery legacy vk random id")
	})
}

func TestChannelNeutralRepositoryScansNullableLegacyAndTargets(t *testing.T) {
	accountID := uuid.New()

	t.Run("job", func(t *testing.T) {
		job := domain.Job{AccountID: uuid.New()}
		if err := scanJob(channelNeutralJobScanRow{accountID: accountID}, &job); err != nil {
			t.Fatalf("scan job: %v", err)
		}
		if job.AccountID != accountID || job.UserID != uuid.Nil || job.VKPeerID != 0 {
			t.Fatalf("job ownership/provenance = %+v", job)
		}
		if job.ChannelContext == nil || job.ChannelContext.Channel != domain.ChannelWeb || job.ChannelContext.RecipientRef != "browser-session" || job.ChannelContext.ThreadRef != "thread-1" {
			t.Fatalf("job channel context = %+v", job.ChannelContext)
		}
		if job.ResultMode != domain.ResultModeAccountHistory || job.DeliveryTarget != nil {
			t.Fatalf("job result contract = %q/%+v", job.ResultMode, job.DeliveryTarget)
		}
	})

	t.Run("delivery", func(t *testing.T) {
		var delivery domain.Delivery
		if err := scanDelivery(channelNeutralDeliveryScanRow{accountID: accountID}, &delivery); err != nil {
			t.Fatalf("scan delivery: %v", err)
		}
		if delivery.AccountID != accountID || delivery.UserID != uuid.Nil || delivery.VKPeerID != 0 || delivery.VKRandomID != 0 {
			t.Fatalf("delivery ownership/provenance = %+v", delivery)
		}
		if delivery.Target == nil || delivery.Target.Channel != domain.ChannelVKBot || delivery.Target.RecipientRef != "peer:2000000001" || delivery.Target.ThreadRef != "thread-1" {
			t.Fatalf("delivery target = %+v", delivery.Target)
		}
	})
}

func TestScanJobClearsNullableAccountIDOnReusedValue(t *testing.T) {
	job := domain.Job{AccountID: uuid.New()}
	if err := scanJob(channelNeutralJobScanRow{}, &job); err != nil {
		t.Fatalf("scan job with null account: %v", err)
	}
	if job.AccountID != uuid.Nil {
		t.Fatalf("nullable account id retained from prior scan: %s", job.AccountID)
	}
}

func TestDeliveryRepositoryRejectsInvalidTargetsBeforeWriting(t *testing.T) {
	ctx := context.Background()
	for _, test := range []struct {
		name string
		run  func(*DeliveryRepository, *domain.Delivery) error
	}{
		{name: "create", run: func(repository *DeliveryRepository, delivery *domain.Delivery) error {
			return repository.Create(ctx, delivery)
		}},
		{name: "update", run: func(repository *DeliveryRepository, delivery *domain.Delivery) error {
			return repository.Update(ctx, delivery)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			recorder := &nullableRecorder{}
			delivery := &domain.Delivery{
				ID:    uuid.New(),
				JobID: uuid.New(),
				Target: &domain.DeliveryTarget{
					Channel:      domain.ChannelWeb,
					RecipientRef: "account",
				},
			}
			err := test.run(NewDeliveryRepository(recorder), delivery)
			if !errors.Is(err, domain.ErrInvalidResultContract) {
				t.Fatalf("%s error = %v, want invalid result contract", test.name, err)
			}
			if len(recorder.args) != 0 {
				t.Fatalf("%s reached database with args %#v", test.name, recorder.args)
			}
		})
	}
}

func TestDeliveryRepositoryUpdatePersistsSafeAccountBackfill(t *testing.T) {
	ctx := context.Background()
	accountID := uuid.New()
	recorder := &nullableRecorder{}
	err := NewDeliveryRepository(recorder).Update(ctx, &domain.Delivery{
		ID:        uuid.New(),
		JobID:     uuid.New(),
		AccountID: accountID,
		Target: &domain.DeliveryTarget{
			Channel:      domain.ChannelVKBot,
			RecipientRef: "555",
		},
	})
	assertNullableRecorderError(t, err)
	if len(recorder.args) < 2 {
		t.Fatalf("delivery update argument count = %d, want account owner binding", len(recorder.args))
	}
	got, ok := recorder.args[1].(*uuid.UUID)
	if !ok || got == nil || *got != accountID {
		t.Fatalf("delivery update account owner argument = %#v, want %s", recorder.args[1], accountID)
	}
	normalizedQuery := strings.Join(strings.Fields(strings.ToLower(recorder.query)), " ")
	if !strings.Contains(normalizedQuery, "account_id = coalesce(account_id, $2)") {
		t.Fatalf("delivery update must only backfill a null account owner, query = %q", normalizedQuery)
	}
}

type channelNeutralJobScanRow struct {
	accountID uuid.UUID
}

func (r channelNeutralJobScanRow) Scan(dest ...any) error {
	if len(dest) < 33 {
		return fmt.Errorf("job scan destination count = %d, want neutral fields", len(dest))
	}
	setNullableUUID(testerDestination(dest, 1), uuid.Nil)
	setNullableUUID(testerDestination(dest, 2), r.accountID)
	setNullableString(testerDestination(dest, 4), "web")
	setNullableString(testerDestination(dest, 5), "browser-session")
	setNullableString(testerDestination(dest, 6), "thread-1")
	setResultMode(testerDestination(dest, 7), domain.ResultModeAccountHistory)
	setNullableString(testerDestination(dest, 8), "")
	setNullableString(testerDestination(dest, 9), "")
	setNullableString(testerDestination(dest, 10), "")
	setNullableInt64(testerDestination(dest, 11), 0)
	setNullableUUID(testerDestination(dest, 12), uuid.Nil)
	return nil
}

type channelNeutralDeliveryScanRow struct {
	accountID uuid.UUID
}

func (r channelNeutralDeliveryScanRow) Scan(dest ...any) error {
	if len(dest) < 21 {
		return fmt.Errorf("delivery scan destination count = %d, want neutral fields", len(dest))
	}
	setNullableUUID(testerDestination(dest, 2), uuid.Nil)
	setNullableUUID(testerDestination(dest, 3), r.accountID)
	setNullableInt64(testerDestination(dest, 4), 0)
	setNullableUUID(testerDestination(dest, 5), uuid.Nil)
	setNullableString(testerDestination(dest, 6), "vk_bot")
	setNullableString(testerDestination(dest, 7), "peer:2000000001")
	setNullableString(testerDestination(dest, 8), "thread-1")
	setNullableInt64(testerDestination(dest, 11), 0)
	return nil
}

func testerDestination(dest []any, index int) any {
	return dest[index]
}

func setNullableUUID(dest any, value uuid.UUID) {
	pointer, ok := dest.(**uuid.UUID)
	if !ok {
		panic(fmt.Sprintf("destination must be **uuid.UUID, got %T", dest))
	}
	if value != uuid.Nil {
		value := value
		*pointer = &value
	}
}

func setNullableString(dest any, value string) {
	pointer, ok := dest.(**string)
	if !ok {
		panic(fmt.Sprintf("destination must be **string, got %T", dest))
	}
	if value != "" {
		value := value
		*pointer = &value
	}
}

func setNullableInt64(dest any, value int64) {
	pointer, ok := dest.(**int64)
	if !ok {
		panic(fmt.Sprintf("destination must be **int64, got %T", dest))
	}
	if value != 0 {
		value := value
		*pointer = &value
	}
}

func setResultMode(dest any, value domain.ResultMode) {
	pointer, ok := dest.(*domain.ResultMode)
	if !ok {
		panic(fmt.Sprintf("destination must be *domain.ResultMode, got %T", dest))
	}
	*pointer = value
}

func assertStringArgument(t *testing.T, args []any, index int, want, label string) {
	t.Helper()
	if len(args) <= index {
		t.Fatalf("%s missing argument %d", label, index)
	}
	got := args[index]
	if pointer, ok := got.(*string); ok && pointer != nil {
		got = *pointer
	}
	if mode, ok := got.(domain.ResultMode); ok {
		got = string(mode)
	}
	if got != want {
		var got any
		if len(args) > index {
			got = args[index]
		}
		t.Fatalf("%s argument = %#v, want %q", label, got, want)
	}
}
