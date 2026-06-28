package server

import (
	"bytes"
	"net/mail"
	"strings"
	"testing"
	"time"

	"ai-shortlink/internal/model"
)

func TestBuildMagicLinkMessageProducesDeliverableHeaders(t *testing.T) {
	st := model.SystemSettings{
		AppName:       "AI短链平台",
		AppNameZH:     "AI短链平台",
		AppNameEN:     "AI Shortlink",
		BaseURL:       "https://s.example.com",
		DefaultLocale: "zh-CN",
	}
	msg, envelopeFrom, err := buildMagicLinkMessage(st, "no-reply@mail.example.com", "admin@example.com", "https://s.example.com/auth/magic/consume?token=abc", time.Date(2026, 6, 28, 10, 30, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("buildMagicLinkMessage() error = %v", err)
	}
	if envelopeFrom != "no-reply@mail.example.com" {
		t.Fatalf("envelope from = %q", envelopeFrom)
	}

	parsed, err := mail.ReadMessage(bytes.NewReader(msg))
	if err != nil {
		t.Fatalf("ReadMessage() error = %v", err)
	}
	headers := parsed.Header
	for _, key := range []string{"From", "To", "Subject", "Date", "Message-Id", "Mime-Version", "Content-Type", "Content-Language"} {
		if headers.Get(key) == "" {
			t.Fatalf("missing %s header in:\n%s", key, msg)
		}
	}

	raw := string(msg)
	checks := []string{
		"Subject: =?UTF-8?",
		"Message-ID: <",
		"@mail.example.com>",
		"Content-Type: multipart/alternative;",
		"Content-Type: text/plain; charset=UTF-8",
		"Content-Type: text/html; charset=UTF-8",
		"Content-Transfer-Encoding: quoted-printable",
		"Auto-Submitted: auto-generated",
		"X-Auto-Response-Suppress: All",
	}
	for _, want := range checks {
		if !strings.Contains(raw, want) {
			t.Fatalf("message missing %q in:\n%s", want, raw)
		}
	}
}

func TestMailDomainPrefersSenderDomain(t *testing.T) {
	st := model.SystemSettings{BaseURL: "http://localhost:8080"}
	if got := mailDomain(st, "no-reply@mail.example.com"); got != "mail.example.com" {
		t.Fatalf("mailDomain() = %q, want sender domain", got)
	}
}
