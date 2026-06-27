package providerbalance

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

type ProviderBalance struct {
	Provider       string
	RemainBalance  float64
	RemainCredits  float64
	UsedBalance    float64
	UsedCredits    float64
	UnlimitedQuota bool
	CheckedAt      time.Time
}

type Checker interface {
	Name() string
	Check(ctx context.Context) (ProviderBalance, error)
}

type Messenger interface {
	SendMessage(ctx context.Context, text string) error
}

type Config struct {
	WarnRemainBalance float64
	WarnRemainCredits float64
	CacheTTL          time.Duration
	Location          *time.Location
}

type Service struct {
	checkers      []Checker
	byName        map[string]Checker
	messenger     Messenger
	cfg           Config
	location      *time.Location
	mu            sync.Mutex
	warningActive map[string]bool
	cacheTTL      time.Duration
	cache         map[string]cachedCheck
}

type cachedCheck struct {
	balance ProviderBalance
	err     error
	expires time.Time
}

func New(checkers []Checker, messenger Messenger, cfg Config) *Service {
	byName := make(map[string]Checker, len(checkers))
	for _, checker := range checkers {
		if checker == nil {
			continue
		}
		byName[normalizeProviderName(checker.Name())] = checker
	}
	location := cfg.Location
	if location == nil {
		var err error
		location, err = time.LoadLocation("Europe/Moscow")
		if err != nil {
			location = time.FixedZone("MSK", 3*60*60)
		}
	}
	return &Service{
		checkers:      append([]Checker(nil), checkers...),
		byName:        byName,
		messenger:     messenger,
		cfg:           cfg,
		location:      location,
		warningActive: map[string]bool{},
		cacheTTL:      cfg.CacheTTL,
		cache:         map[string]cachedCheck{},
	}
}

func (s *Service) HandleCommand(ctx context.Context, text string) error {
	command, args := parseCommand(text)
	switch command {
	case "/balances":
		return s.send(ctx, s.renderAllBalances(ctx))
	case "/balance":
		if len(args) == 1 {
			if checker, ok := s.byName[normalizeProviderName(args[0])]; ok {
				return s.send(ctx, s.renderChecker(ctx, checker))
			}
		}
		return s.send(ctx, helpMessage())
	case "/help":
		return s.send(ctx, helpMessage())
	default:
		return s.send(ctx, helpMessage())
	}
}

func (s *Service) CheckAndWarn(ctx context.Context) error {
	for _, checker := range s.checkers {
		if checker == nil {
			continue
		}
		balance, err := s.check(ctx, checker)
		if err != nil {
			continue
		}
		provider := providerKey(checker, balance)
		low, warning := s.warningFor(balance)
		if !low {
			s.setWarningActive(provider, false)
			continue
		}
		if s.isWarningActive(provider) {
			continue
		}
		if err := s.send(ctx, warning); err != nil {
			return err
		}
		s.setWarningActive(provider, true)
	}
	return nil
}

func (s *Service) renderAllBalances(ctx context.Context) string {
	var b strings.Builder
	b.WriteString("Балансы провайдеров")
	for _, checker := range s.checkers {
		if checker == nil {
			continue
		}
		b.WriteString("\n\n")
		b.WriteString(s.renderChecker(ctx, checker))
	}
	return b.String()
}

func (s *Service) renderChecker(ctx context.Context, checker Checker) string {
	balance, err := s.check(ctx, checker)
	if err != nil {
		return renderProviderError(checker.Name(), err)
	}
	return s.renderBalance(balance, checker.Name())
}

func (s *Service) check(ctx context.Context, checker Checker) (ProviderBalance, error) {
	if s.cacheTTL <= 0 {
		return checker.Check(ctx)
	}
	key := normalizeProviderName(checker.Name())
	now := time.Now()
	s.mu.Lock()
	if cached, ok := s.cache[key]; ok && now.Before(cached.expires) {
		s.mu.Unlock()
		return cached.balance, cached.err
	}
	s.mu.Unlock()

	balance, err := checker.Check(ctx)
	if err != nil && ctx.Err() != nil {
		return balance, err
	}

	s.mu.Lock()
	s.cache[key] = cachedCheck{
		balance: balance,
		err:     err,
		expires: time.Now().Add(s.cacheTTL),
	}
	s.mu.Unlock()
	return balance, err
}

func (s *Service) renderBalance(balance ProviderBalance, fallbackProvider string) string {
	checkedAt := balance.CheckedAt
	if checkedAt.IsZero() {
		checkedAt = time.Now()
	}
	checkedAt = checkedAt.In(s.location)
	return fmt.Sprintf("%s\nОстаток: %.2f balance\nОстаток credits: %.1f\nИспользовано: %.2f balance\nИспользовано credits: %.1f\nСтатус: ok\nОбновлено: %s",
		displayProviderName(providerName(balance, fallbackProvider)),
		balance.RemainBalance,
		balance.RemainCredits,
		balance.UsedBalance,
		balance.UsedCredits,
		checkedAt.Format("2006-01-02 15:04 MST"),
	)
}

func (s *Service) warningFor(balance ProviderBalance) (bool, string) {
	name := displayProviderName(providerName(balance, ""))
	if s.cfg.WarnRemainBalance > 0 && balance.RemainBalance < s.cfg.WarnRemainBalance {
		return true, fmt.Sprintf("Низкий баланс провайдера\n\n%s\nОстаток: %.2f balance\nПорог: %s balance\n\nПополните %s, чтобы генерации не начали падать по балансу провайдера",
			name,
			balance.RemainBalance,
			formatThreshold(s.cfg.WarnRemainBalance),
			name,
		)
	}
	if s.cfg.WarnRemainCredits > 0 && balance.RemainCredits < s.cfg.WarnRemainCredits {
		return true, fmt.Sprintf("Низкий баланс провайдера\n\n%s\nОстаток credits: %.1f\nПорог: %s credits\n\nПополните %s, чтобы генерации не начали падать по балансу провайдера",
			name,
			balance.RemainCredits,
			formatThreshold(s.cfg.WarnRemainCredits),
			name,
		)
	}
	return false, ""
}

func (s *Service) send(ctx context.Context, text string) error {
	if s.messenger == nil {
		return fmt.Errorf("providerbalance: messenger is not configured")
	}
	return s.messenger.SendMessage(ctx, text)
}

func (s *Service) isWarningActive(provider string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.warningActive[provider]
}

func (s *Service) setWarningActive(provider string, active bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if active {
		s.warningActive[provider] = true
		return
	}
	delete(s.warningActive, provider)
}

func helpMessage() string {
	return "Команды:\n/balances\n/balance apimart\n/balance poyo\n/balance runway\n/balance deepinfra\n/help"
}

func renderProviderError(provider string, err error) string {
	return fmt.Sprintf("Не удалось получить баланс %s\n\nСтатус: provider_unavailable\nОшибка: %s",
		displayProviderName(provider),
		safeErrorMessage(err),
	)
}

func parseCommand(text string) (string, []string) {
	fields := strings.Fields(strings.TrimSpace(text))
	if len(fields) == 0 {
		return "", nil
	}
	command := strings.ToLower(fields[0])
	if at := strings.IndexByte(command, '@'); at >= 0 {
		command = command[:at]
	}
	return command, fields[1:]
}

func providerKey(checker Checker, balance ProviderBalance) string {
	return normalizeProviderName(providerName(balance, checker.Name()))
}

func providerName(balance ProviderBalance, fallback string) string {
	if strings.TrimSpace(balance.Provider) != "" {
		return balance.Provider
	}
	return fallback
}

func normalizeProviderName(provider string) string {
	return strings.ToLower(strings.TrimSpace(provider))
}

func displayProviderName(provider string) string {
	switch normalizeProviderName(provider) {
	case "apimart":
		return "APIMart"
	case "poyo":
		return "PoYo"
	case "runway":
		return "Runway"
	case "deepinfra":
		return "DeepInfra"
	default:
		provider = strings.TrimSpace(provider)
		if provider == "" {
			return "Provider"
		}
		return provider
	}
}

func formatThreshold(value float64) string {
	if value == float64(int64(value)) {
		return fmt.Sprintf("%.0f", value)
	}
	return fmt.Sprintf("%.2f", value)
}

var (
	urlPattern    = regexp.MustCompile(`(?i)(https?://|data:)\S+`)
	authPattern   = regexp.MustCompile(`(?i)authorization:\s*bearer\s+\S+`)
	bearerPattern = regexp.MustCompile(`(?i)bearer\s+\S+`)
	emailPattern  = regexp.MustCompile(`(?i)[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}`)
)

func safeErrorMessage(err error) string {
	if err == nil {
		return "provider unavailable"
	}
	message := strings.TrimSpace(err.Error())
	if message == "" {
		return "provider unavailable"
	}
	message = authPattern.ReplaceAllString(message, "[redacted-auth]")
	message = bearerPattern.ReplaceAllString(message, "Bearer [redacted]")
	message = urlPattern.ReplaceAllString(message, "[redacted-url]")
	message = emailPattern.ReplaceAllString(message, "[redacted-email]")
	message = strings.Join(strings.Fields(message), " ")
	if len([]rune(message)) > 240 {
		runes := []rune(message)
		message = string(runes[:240])
	}
	return message
}
