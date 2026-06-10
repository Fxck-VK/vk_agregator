// Package antispam protects VK bot intake from user-level floods before
// expensive jobs are created. It stores counters in Redis-compatible storage so
// limits work across API instances.
package antispam

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

const (
	keyMessages   = "messages"
	keyGPT        = "gpt"
	keyImageDaily = "image_daily"
	keyViolations = "violations"
	keyBlock      = "block"
	keyCooldown   = "cooldown"
)

// Store is the minimal Redis-like contract required by anti-spam counters.
type Store interface {
	Increment(ctx context.Context, key string, window time.Duration) (count int64, ttl time.Duration, err error)
	TTL(ctx context.Context, key string) (time.Duration, error)
	SetTTL(ctx context.Context, key string, ttl time.Duration) error
}

// ActiveJobCounter counts still-active jobs for one user and operation.
type ActiveJobCounter interface {
	CountActiveByUserOperation(ctx context.Context, userID uuid.UUID, operation domain.OperationType) (int, error)
}

// Config tunes VK anti-spam behavior.
type Config struct {
	Enabled bool

	MessageLimit     int
	MessageWindow    time.Duration
	GPTLimit         int
	GPTWindow        time.Duration
	ImageDailyLimit  int
	ImageDailyWindow time.Duration

	Cooldown        time.Duration
	ViolationLimit  int
	ViolationWindow time.Duration
	BlockDuration   time.Duration

	NewUserAge          time.Duration
	NewUserMessageLimit int
	NewUserGPTLimit     int
	NewUserGPTWindow    time.Duration

	ActiveGPTJobLimit int
}

// DefaultConfig returns the product defaults requested for the VK bot.
func DefaultConfig() Config {
	return Config{
		Enabled:             true,
		MessageLimit:        10,
		MessageWindow:       time.Minute,
		GPTLimit:            3,
		GPTWindow:           30 * time.Second,
		ImageDailyLimit:     100,
		ImageDailyWindow:    24 * time.Hour,
		Cooldown:            30 * time.Second,
		ViolationLimit:      5,
		ViolationWindow:     10 * time.Minute,
		BlockDuration:       15 * time.Minute,
		NewUserAge:          4 * time.Hour,
		NewUserMessageLimit: 5,
		NewUserGPTLimit:     1,
		NewUserGPTWindow:    15 * time.Second,
		ActiveGPTJobLimit:   2,
	}
}

// CheckInput describes a normalized VK event after user resolution and command
// parsing, but before command/job persistence.
type CheckInput struct {
	User        *domain.User
	VKUserID    int64
	CommandType domain.CommandType
	Operation   domain.OperationType
	CreatesJob  bool
}

// DecisionKind explains why an event was allowed or denied.
type DecisionKind string

const (
	DecisionAllow          DecisionKind = "allow"
	DecisionCooldown       DecisionKind = "cooldown"
	DecisionTemporaryBlock DecisionKind = "temporary_block"
	DecisionActiveJobs     DecisionKind = "active_jobs"
	DecisionImageDaily     DecisionKind = "image_daily_limit"
)

// Decision is the result of an anti-spam check.
type Decision struct {
	Allowed    bool
	Kind       DecisionKind
	RetryAfter time.Duration
	Message    string
}

// Service implements VK user-level anti-spam limits.
type Service struct {
	store Store
	jobs  ActiveJobCounter
	cfg   Config
	now   func() time.Time
}

// New builds an anti-spam service.
func New(store Store, jobs ActiveJobCounter, cfg Config) *Service {
	cfg = normalizeConfig(cfg)
	return &Service{
		store: store,
		jobs:  jobs,
		cfg:   cfg,
		now:   time.Now,
	}
}

// WithClock overrides the clock for deterministic tests.
func (s *Service) WithClock(now func() time.Time) *Service {
	if now != nil {
		s.now = now
	}
	return s
}

// Check evaluates message, GPT and active-job limits. It fails open only when
// callers choose to ignore the returned error; the service itself reports store
// failures so handlers can log them.
func (s *Service) Check(ctx context.Context, in CheckInput) (Decision, error) {
	if !s.cfg.Enabled || s.store == nil || in.User == nil || in.VKUserID == 0 {
		return allow(), nil
	}

	keys := keysForUser(in.VKUserID)
	if ttl, err := s.store.TTL(ctx, keys[keyBlock]); err != nil {
		return allow(), fmt.Errorf("antispam block ttl: %w", err)
	} else if ttl > 0 {
		return temporaryBlock(ttl), nil
	}

	if ttl, err := s.store.TTL(ctx, keys[keyCooldown]); err != nil {
		return allow(), fmt.Errorf("antispam cooldown ttl: %w", err)
	} else if ttl > 0 {
		return s.registerViolation(ctx, keys, ttl, false)
	}

	profile := s.profileFor(in.User)
	msgCount, msgTTL, err := s.store.Increment(ctx, keys[keyMessages], profile.messageWindow)
	if err != nil {
		return allow(), fmt.Errorf("antispam message increment: %w", err)
	}
	if msgCount > int64(profile.messageLimit) {
		return s.registerViolation(ctx, keys, minPositiveDuration(s.cfg.Cooldown, msgTTL), true)
	}

	if in.CreatesJob && in.Operation == domain.OperationTextGenerate {
		if s.cfg.ActiveGPTJobLimit > 0 && s.jobs != nil {
			active, err := s.jobs.CountActiveByUserOperation(ctx, in.User.ID, domain.OperationTextGenerate)
			if err != nil {
				return allow(), fmt.Errorf("antispam active job count: %w", err)
			}
			if active >= s.cfg.ActiveGPTJobLimit {
				return activeJobs(), nil
			}
		}

		gptCount, gptTTL, err := s.store.Increment(ctx, keys[keyGPT], profile.gptWindow)
		if err != nil {
			return allow(), fmt.Errorf("antispam gpt increment: %w", err)
		}
		if gptCount > int64(profile.gptLimit) {
			return s.registerViolation(ctx, keys, minPositiveDuration(s.cfg.Cooldown, gptTTL), true)
		}
	}
	if in.CreatesJob && in.Operation == domain.OperationImageGenerate && profile.imageDailyLimit > 0 {
		imageCount, imageTTL, err := s.store.Increment(ctx, keys[keyImageDaily], profile.imageDailyWindow)
		if err != nil {
			return allow(), fmt.Errorf("antispam image daily increment: %w", err)
		}
		if imageCount > int64(profile.imageDailyLimit) {
			return imageDailyLimit(imageTTL), nil
		}
	}

	return allow(), nil
}

type limitProfile struct {
	messageLimit     int
	messageWindow    time.Duration
	gptLimit         int
	gptWindow        time.Duration
	imageDailyLimit  int
	imageDailyWindow time.Duration
}

func (s *Service) profileFor(user *domain.User) limitProfile {
	profile := limitProfile{
		messageLimit:     s.cfg.MessageLimit,
		messageWindow:    s.cfg.MessageWindow,
		gptLimit:         s.cfg.GPTLimit,
		gptWindow:        s.cfg.GPTWindow,
		imageDailyLimit:  s.cfg.ImageDailyLimit,
		imageDailyWindow: s.cfg.ImageDailyWindow,
	}
	if s.isNewUser(user) {
		profile.messageLimit = s.cfg.NewUserMessageLimit
		profile.gptLimit = s.cfg.NewUserGPTLimit
		profile.gptWindow = s.cfg.NewUserGPTWindow
	}
	return profile
}

func (s *Service) isNewUser(user *domain.User) bool {
	if s.cfg.NewUserAge <= 0 {
		return false
	}
	seenAt := user.FirstSeenAt
	if seenAt.IsZero() {
		seenAt = user.CreatedAt
	}
	if seenAt.IsZero() {
		return true
	}
	return s.now().Sub(seenAt) < s.cfg.NewUserAge
}

func (s *Service) registerViolation(ctx context.Context, keys map[string]string, retryAfter time.Duration, setCooldown bool) (Decision, error) {
	count, _, err := s.store.Increment(ctx, keys[keyViolations], s.cfg.ViolationWindow)
	if err != nil {
		return allow(), fmt.Errorf("antispam violation increment: %w", err)
	}
	if count >= int64(s.cfg.ViolationLimit) {
		if err := s.store.SetTTL(ctx, keys[keyBlock], s.cfg.BlockDuration); err != nil {
			return allow(), fmt.Errorf("antispam set block: %w", err)
		}
		return temporaryBlock(s.cfg.BlockDuration), nil
	}
	if setCooldown && retryAfter > 0 {
		if err := s.store.SetTTL(ctx, keys[keyCooldown], retryAfter); err != nil {
			return allow(), fmt.Errorf("antispam set cooldown: %w", err)
		}
	}
	return cooldown(retryAfter), nil
}

func keysForUser(vkUserID int64) map[string]string {
	return map[string]string{
		keyMessages:   fmt.Sprintf("rate:vk:user:%d:messages", vkUserID),
		keyGPT:        fmt.Sprintf("rate:vk:user:%d:gpt", vkUserID),
		keyImageDaily: fmt.Sprintf("rate:vk:user:%d:image_daily", vkUserID),
		keyViolations: fmt.Sprintf("spam:vk:user:%d:violations", vkUserID),
		keyBlock:      fmt.Sprintf("block:vk:user:%d", vkUserID),
		keyCooldown:   fmt.Sprintf("cooldown:vk:user:%d", vkUserID),
	}
}

func normalizeConfig(cfg Config) Config {
	def := DefaultConfig()
	if cfg.MessageLimit <= 0 {
		cfg.MessageLimit = def.MessageLimit
	}
	if cfg.MessageWindow <= 0 {
		cfg.MessageWindow = def.MessageWindow
	}
	if cfg.GPTLimit <= 0 {
		cfg.GPTLimit = def.GPTLimit
	}
	if cfg.GPTWindow <= 0 {
		cfg.GPTWindow = def.GPTWindow
	}
	if cfg.ImageDailyLimit < 0 {
		cfg.ImageDailyLimit = def.ImageDailyLimit
	}
	if cfg.ImageDailyWindow <= 0 {
		cfg.ImageDailyWindow = def.ImageDailyWindow
	}
	if cfg.Cooldown <= 0 {
		cfg.Cooldown = def.Cooldown
	}
	if cfg.ViolationLimit <= 0 {
		cfg.ViolationLimit = def.ViolationLimit
	}
	if cfg.ViolationWindow <= 0 {
		cfg.ViolationWindow = def.ViolationWindow
	}
	if cfg.BlockDuration <= 0 {
		cfg.BlockDuration = def.BlockDuration
	}
	if cfg.NewUserAge < 0 {
		cfg.NewUserAge = def.NewUserAge
	}
	if cfg.NewUserMessageLimit <= 0 {
		cfg.NewUserMessageLimit = def.NewUserMessageLimit
	}
	if cfg.NewUserGPTLimit <= 0 {
		cfg.NewUserGPTLimit = def.NewUserGPTLimit
	}
	if cfg.NewUserGPTWindow <= 0 {
		cfg.NewUserGPTWindow = def.NewUserGPTWindow
	}
	if cfg.ActiveGPTJobLimit < 0 {
		cfg.ActiveGPTJobLimit = def.ActiveGPTJobLimit
	}
	return cfg
}

func allow() Decision {
	return Decision{Allowed: true, Kind: DecisionAllow}
}

func cooldown(ttl time.Duration) Decision {
	seconds := ceilDuration(ttl, time.Second)
	return Decision{
		Allowed:    false,
		Kind:       DecisionCooldown,
		RetryAfter: time.Duration(seconds) * time.Second,
		Message:    fmt.Sprintf("Слишком много сообщений. Попробуйте через %d секунд", seconds),
	}
}

func temporaryBlock(ttl time.Duration) Decision {
	minutes := ceilDuration(ttl, time.Minute)
	return Decision{
		Allowed:    false,
		Kind:       DecisionTemporaryBlock,
		RetryAfter: time.Duration(minutes) * time.Minute,
		Message:    fmt.Sprintf("Доступ временно ограничен из-за слишком частых запросов\n\nПопробуйте через %d минут", minutes),
	}
}

func activeJobs() Decision {
	return Decision{
		Allowed: false,
		Kind:    DecisionActiveJobs,
		Message: "У вас уже есть активный запрос\n\nПожалуйста, дождитесь ответа",
	}
}

func imageDailyLimit(ttl time.Duration) Decision {
	hours := ceilDuration(ttl, time.Hour)
	return Decision{
		Allowed:    false,
		Kind:       DecisionImageDaily,
		RetryAfter: time.Duration(hours) * time.Hour,
		Message:    fmt.Sprintf("Лимит генерации фото на сегодня исчерпан. Попробуйте через %d ч.", hours),
	}
}

func ceilDuration(d, unit time.Duration) int64 {
	if d <= 0 {
		return 1
	}
	return int64(math.Ceil(float64(d) / float64(unit)))
}

func minPositiveDuration(a, b time.Duration) time.Duration {
	if a <= 0 {
		return b
	}
	if b <= 0 {
		return a
	}
	if a < b {
		return a
	}
	return b
}
