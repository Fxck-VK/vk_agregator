package accountdelivery

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/accountlink"
)

func TestSecurityNoticeDisabledFailsClosed(t *testing.T) {
	sender, _ := NewSender(Config{})
	if !errors.Is(sender.SendAccountSecurityNotice(context.Background(), domain.AccountSecurityNotice{}), accountlink.ErrDeliveryUnavailable) {
		t.Fatal("disabled notice sender did not fail closed")
	}
}

func TestSecurityNoticeSMTPUsesFakeServer(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	received := make(chan string, 1)
	serverErr := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			serverErr <- err
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
		reader := bufio.NewReader(conn)
		if _, err = io.WriteString(conn, "220 fake SMTP\r\n"); err != nil {
			serverErr <- err
			return
		}
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				serverErr <- err
				return
			}
			switch {
			case strings.HasPrefix(line, "EHLO"):
				_, err = io.WriteString(conn, "250 fake\r\n")
			case strings.HasPrefix(line, "MAIL FROM:"), strings.HasPrefix(line, "RCPT TO:"):
				_, err = io.WriteString(conn, "250 OK\r\n")
			case strings.HasPrefix(line, "DATA"):
				_, err = io.WriteString(conn, "354 body\r\n")
				if err != nil {
					serverErr <- err
					return
				}
				var body strings.Builder
				for {
					part, readErr := reader.ReadString('\n')
					if readErr != nil {
						serverErr <- readErr
						return
					}
					if part == ".\r\n" {
						break
					}
					body.WriteString(part)
				}
				received <- body.String()
				_, err = io.WriteString(conn, "250 accepted\r\n")
			case strings.HasPrefix(line, "QUIT"):
				_, err = io.WriteString(conn, "221 bye\r\n")
				serverErr <- err
				return
			default:
				serverErr <- errors.New("unexpected fake SMTP command")
				return
			}
			if err != nil {
				serverErr <- err
				return
			}
		}
	}()
	host, portString, _ := net.SplitHostPort(listener.Addr().String())
	port, _ := strconv.Atoi(portString)
	sender, err := NewSender(Config{EmailProvider: ProviderSMTP, EmailSMTP: SMTPConfig{Host: host, Port: port, From: "noreply@example.test", TLSMode: "none", Timeout: time.Second}})
	if err != nil {
		t.Fatal(err)
	}
	if err := sender.SendAccountSecurityNotice(context.Background(), domain.AccountSecurityNotice{Recipient: "fixture@example.test", Kind: domain.AccountSecurityNoticePasswordChanged, EventAt: time.Now()}); err != nil {
		t.Fatal("fake SMTP send failed")
	}
	if err := <-serverErr; err != nil {
		t.Fatal("fake SMTP server failed")
	}
	if !strings.Contains(<-received, "Пароль аккаунта изменён.") {
		t.Fatal("wrong template delivered")
	}
}

func TestSecurityNoticeSMTPHonorsContextDeadline(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(time.Second))
		_, _ = io.Copy(io.Discard, conn)
	}()
	host, portString, _ := net.SplitHostPort(listener.Addr().String())
	port, _ := strconv.Atoi(portString)
	sender, err := NewSender(Config{EmailProvider: ProviderSMTP, EmailSMTP: SMTPConfig{Host: host, Port: port, From: "noreply@example.test", TLSMode: "none", Timeout: 10 * time.Second}})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	if err := sender.SendAccountSecurityNotice(ctx, domain.AccountSecurityNotice{Recipient: "fixture@example.test", Kind: domain.AccountSecurityNoticePasswordReset, EventAt: start}); err == nil {
		t.Fatal("silent fake SMTP unexpectedly succeeded")
	}
	if time.Since(start) > time.Second {
		t.Fatal("SMTP ignored context deadline")
	}
	<-done
}

func TestSecurityNoticeFixedRussianTemplates(t *testing.T) {
	for _, kind := range []domain.AccountSecurityNoticeKind{domain.AccountSecurityNoticePasswordChanged, domain.AccountSecurityNoticePasswordReset, domain.AccountSecurityNoticeBackupEmailAdded, domain.AccountSecurityNoticeBackupEmailReplaced, domain.AccountSecurityNoticeEmailRemoved} {
		msg, err := buildSecurityNoticeMessage("noreply@example.test", "user@example.test", kind, time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC))
		if err != nil {
			t.Fatal(err)
		}
		body := strings.SplitN(string(msg), "\r\n\r\n", 2)[1]
		if !strings.Contains(body, "НейроХаб") || !strings.Contains(body, "2026-10-05") {
			t.Fatal("fixed Russian body missing")
		}
		for _, forbidden := range []string{"http", "123456", "token", "example.test"} {
			if strings.Contains(body, forbidden) {
				t.Fatal("body contains sensitive data")
			}
		}
	}
	if _, err := buildSecurityNoticeMessage("a", "b", domain.AccountSecurityNoticeKind("arbitrary"), time.Now()); err == nil {
		t.Fatal("unknown event accepted")
	}
}
