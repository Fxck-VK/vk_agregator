// Package accountdelivery sends account ownership verification codes.
package accountdelivery

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"vk-ai-aggregator/internal/service/accountlink"
)

const (
	ProviderDisabled = "disabled"
	ProviderSMTP     = "smtp"
	ProviderHTTP     = "http"

	defaultEmailSubject = "Код подтверждения НейроХаб"
	defaultTimeout      = 10 * time.Second
)

// Config describes account verification delivery adapters.
type Config struct {
	EmailProvider string
	EmailSMTP     SMTPConfig

	PhoneProvider string
	PhoneHTTP     HTTPPhoneConfig
}

// SMTPConfig configures email code delivery.
type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	Subject  string
	TLSMode  string
	Timeout  time.Duration
}

// HTTPPhoneConfig configures generic HTTP SMS/OTP delivery.
type HTTPPhoneConfig struct {
	URL             string
	Method          string
	AuthHeaderName  string
	AuthHeaderValue string
	ContentType     string
	BodyTemplate    string
	Timeout         time.Duration
}

type emailSender interface {
	SendEmailLinkCode(ctx context.Context, email, code string, expiresAt time.Time) error
}

type phoneSender interface {
	SendPhoneLinkOTP(ctx context.Context, phone, code string, expiresAt time.Time) error
}

// Sender implements accountlink.Sender with independently configurable
// email and phone providers.
type Sender struct {
	email emailSender
	phone phoneSender
}

// NewSender builds a fail-closed account verification sender.
func NewSender(cfg Config) (*Sender, error) {
	emailProvider := normalizedProvider(cfg.EmailProvider)
	phoneProvider := normalizedProvider(cfg.PhoneProvider)

	sender := &Sender{}
	switch emailProvider {
	case "", ProviderDisabled:
	case ProviderSMTP:
		email, err := newSMTPSender(cfg.EmailSMTP)
		if err != nil {
			return nil, err
		}
		sender.email = email
	default:
		return nil, fmt.Errorf("accountdelivery: unsupported email provider %q", cfg.EmailProvider)
	}

	switch phoneProvider {
	case "", ProviderDisabled:
	case ProviderHTTP:
		phone, err := newHTTPPhoneSender(cfg.PhoneHTTP)
		if err != nil {
			return nil, err
		}
		sender.phone = phone
	default:
		return nil, fmt.Errorf("accountdelivery: unsupported phone provider %q", cfg.PhoneProvider)
	}

	return sender, nil
}

// SendEmailLinkCode delivers an email code, or fails closed when disabled.
func (s *Sender) SendEmailLinkCode(ctx context.Context, email, code string, expiresAt time.Time) error {
	if s == nil || s.email == nil {
		return accountlink.ErrDeliveryUnavailable
	}
	return s.email.SendEmailLinkCode(ctx, email, code, expiresAt)
}

// SendPhoneLinkOTP delivers a phone OTP, or fails closed when disabled.
func (s *Sender) SendPhoneLinkOTP(ctx context.Context, phone, code string, expiresAt time.Time) error {
	if s == nil || s.phone == nil {
		return accountlink.ErrSMSDeliveryUnavailable
	}
	return s.phone.SendPhoneLinkOTP(ctx, phone, code, expiresAt)
}

type smtpSender struct {
	cfg SMTPConfig
}

func newSMTPSender(cfg SMTPConfig) (*smtpSender, error) {
	cfg.Host = strings.TrimSpace(cfg.Host)
	cfg.From = strings.TrimSpace(cfg.From)
	cfg.Subject = strings.TrimSpace(cfg.Subject)
	cfg.TLSMode = strings.ToLower(strings.TrimSpace(cfg.TLSMode))
	if cfg.Port == 0 {
		cfg.Port = 587
	}
	if cfg.Subject == "" {
		cfg.Subject = "NeiroHub verification code"
	}
	if cfg.TLSMode == "" {
		cfg.TLSMode = "starttls"
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = defaultTimeout
	}
	if cfg.Host == "" || cfg.From == "" {
		return nil, errors.New("accountdelivery: smtp host and from are required")
	}
	if cfg.Port < 1 || cfg.Port > 65535 {
		return nil, errors.New("accountdelivery: smtp port is invalid")
	}
	if cfg.TLSMode != "starttls" && cfg.TLSMode != "none" {
		return nil, errors.New("accountdelivery: smtp tls mode must be starttls or none")
	}
	if (strings.TrimSpace(cfg.Username) == "") != (strings.TrimSpace(cfg.Password) == "") {
		return nil, errors.New("accountdelivery: smtp username and password must be set together")
	}
	return &smtpSender{cfg: cfg}, nil
}

func (s *smtpSender) SendEmailLinkCode(ctx context.Context, email, code string, expiresAt time.Time) error {
	message := buildEmailMessage(s.cfg.From, email, s.cfg.Subject, code, expiresAt)
	addr := net.JoinHostPort(s.cfg.Host, strconv.Itoa(s.cfg.Port))
	dialer := net.Dialer{Timeout: s.cfg.Timeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("accountdelivery: smtp connect failed: %w", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(s.cfg.Timeout))

	client, err := smtp.NewClient(conn, s.cfg.Host)
	if err != nil {
		return fmt.Errorf("accountdelivery: smtp client failed: %w", err)
	}
	defer client.Close()

	if s.cfg.TLSMode == "starttls" {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return errors.New("accountdelivery: smtp starttls unavailable")
		}
		tlsCfg := &tls.Config{ServerName: s.cfg.Host, MinVersion: tls.VersionTLS12}
		if err := client.StartTLS(tlsCfg); err != nil {
			return fmt.Errorf("accountdelivery: smtp starttls failed: %w", err)
		}
	}
	if strings.TrimSpace(s.cfg.Username) != "" {
		auth := smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("accountdelivery: smtp auth failed: %w", err)
		}
	}
	if err := client.Mail(s.cfg.From); err != nil {
		return fmt.Errorf("accountdelivery: smtp mail failed: %w", err)
	}
	if err := client.Rcpt(email); err != nil {
		return fmt.Errorf("accountdelivery: smtp recipient failed: %w", err)
	}
	wc, err := client.Data()
	if err != nil {
		return fmt.Errorf("accountdelivery: smtp data failed: %w", err)
	}
	if _, err := wc.Write(message); err != nil {
		_ = wc.Close()
		return fmt.Errorf("accountdelivery: smtp write failed: %w", err)
	}
	if err := wc.Close(); err != nil {
		return fmt.Errorf("accountdelivery: smtp close failed: %w", err)
	}
	if err := client.Quit(); err != nil {
		return fmt.Errorf("accountdelivery: smtp quit failed: %w", err)
	}
	return nil
}

type httpPhoneSender struct {
	cfg    HTTPPhoneConfig
	client *http.Client
}

func newHTTPPhoneSender(cfg HTTPPhoneConfig) (*httpPhoneSender, error) {
	cfg.URL = strings.TrimSpace(cfg.URL)
	cfg.Method = strings.ToUpper(strings.TrimSpace(cfg.Method))
	cfg.AuthHeaderName = strings.TrimSpace(cfg.AuthHeaderName)
	cfg.ContentType = strings.TrimSpace(cfg.ContentType)
	if cfg.Method == "" {
		cfg.Method = http.MethodPost
	}
	if cfg.ContentType == "" {
		cfg.ContentType = "application/json"
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = defaultTimeout
	}
	if cfg.URL == "" || strings.TrimSpace(cfg.BodyTemplate) == "" {
		return nil, errors.New("accountdelivery: phone http url and body template are required")
	}
	if cfg.Method != http.MethodPost && cfg.Method != http.MethodPut && cfg.Method != http.MethodPatch {
		return nil, errors.New("accountdelivery: phone http method must be POST, PUT, or PATCH")
	}
	if (cfg.AuthHeaderName == "") != (strings.TrimSpace(cfg.AuthHeaderValue) == "") {
		return nil, errors.New("accountdelivery: phone http auth header name and value must be set together")
	}
	if !strings.Contains(cfg.BodyTemplate, "{{phone}}") || !strings.Contains(cfg.BodyTemplate, "{{code}}") {
		return nil, errors.New("accountdelivery: phone http body template must include {{phone}} and {{code}}")
	}
	return &httpPhoneSender{
		cfg:    cfg,
		client: &http.Client{Timeout: cfg.Timeout},
	}, nil
}

func (s *httpPhoneSender) SendPhoneLinkOTP(ctx context.Context, phone, code string, expiresAt time.Time) error {
	body := renderPhoneBody(s.cfg.BodyTemplate, phone, code, expiresAt)
	req, err := http.NewRequestWithContext(ctx, s.cfg.Method, s.cfg.URL, strings.NewReader(body))
	if err != nil {
		return fmt.Errorf("accountdelivery: phone request failed: %w", err)
	}
	req.Header.Set("Content-Type", s.cfg.ContentType)
	if s.cfg.AuthHeaderName != "" {
		req.Header.Set(s.cfg.AuthHeaderName, s.cfg.AuthHeaderValue)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("accountdelivery: phone provider failed: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 8<<10))
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("accountdelivery: phone provider returned status %d", resp.StatusCode)
	}
	return nil
}

func normalizedProvider(provider string) string {
	return strings.ToLower(strings.TrimSpace(provider))
}

func buildEmailMessageLegacy(from, to, subject, code string, expiresAt time.Time) []byte {
	var buf bytes.Buffer
	headers := map[string]string{
		"From":                      sanitizeHeader(from),
		"To":                        sanitizeHeader(to),
		"Subject":                   mime.QEncoding.Encode("utf-8", sanitizeHeader(subject)),
		"MIME-Version":              "1.0",
		"Content-Type":              "text/plain; charset=UTF-8",
		"Content-Transfer-Encoding": "8bit",
	}
	for key, value := range headers {
		_, _ = fmt.Fprintf(&buf, "%s: %s\r\n", key, value)
	}
	_, _ = fmt.Fprintf(&buf, "\r\nКод подтверждения НейроХаб: %s\r\n", code)
	_, _ = fmt.Fprintf(&buf, "Код действует до %s.\r\n", expiresAt.UTC().Format(time.RFC3339))
	_, _ = fmt.Fprint(&buf, "Если вы не запрашивали код, просто проигнорируйте это письмо.\r\n")
	return buf.Bytes()
}

func buildEmailMessage(from, to, subject, code string, expiresAt time.Time) []byte {
	var buf bytes.Buffer
	headers := map[string]string{
		"From":                      sanitizeHeader(from),
		"To":                        sanitizeHeader(to),
		"Subject":                   mime.QEncoding.Encode("utf-8", sanitizeHeader(subject)),
		"MIME-Version":              "1.0",
		"Content-Type":              "text/plain; charset=UTF-8",
		"Content-Transfer-Encoding": "8bit",
	}
	for key, value := range headers {
		_, _ = fmt.Fprintf(&buf, "%s: %s\r\n", key, value)
	}
	_, _ = fmt.Fprintf(&buf, "\r\nNeiroHub verification code: %s\r\n", code)
	_, _ = fmt.Fprintf(&buf, "This code expires at %s.\r\n", expiresAt.UTC().Format(time.RFC3339))
	_, _ = fmt.Fprint(&buf, "If you did not request this code, ignore this email.\r\n")
	return buf.Bytes()
}

func renderPhoneBody(template, phone, code string, expiresAt time.Time) string {
	replacements := map[string]string{
		"{{phone}}":              phone,
		"{{code}}":               code,
		"{{expires_at}}":         expiresAt.UTC().Format(time.RFC3339),
		"{{expires_in_minutes}}": strconv.FormatInt(maxInt64(1, int64(time.Until(expiresAt).Minutes())), 10),
	}
	out := template
	for old, value := range replacements {
		out = strings.ReplaceAll(out, old, value)
	}
	return out
}

func sanitizeHeader(value string) string {
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	return strings.TrimSpace(value)
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
