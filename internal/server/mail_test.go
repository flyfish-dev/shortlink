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

func TestBuildMagicLinkMessageUsesLocalizedTextTemplate(t *testing.T) {
	st := model.SystemSettings{
		AppName:       "AI Shortlink",
		AppNameZH:     "AI短链平台",
		AppNameEN:     "AI Shortlink",
		BaseURL:       "https://s.example.com",
		DefaultLocale: "en-US",
	}
	msg, _, err := buildMagicLinkMessage(st, "no-reply@mail.example.com", "admin@example.com", "https://s.example.com/auth/magic/consume?token=abc", time.Date(2026, 6, 28, 10, 30, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("buildMagicLinkMessage() error = %v", err)
	}
	raw := string(msg)
	if !strings.Contains(raw, "Your one-time login link for AI Shortlink") {
		t.Fatalf("English magic link body missing localized intro in:\n%s", raw)
	}
	if strings.Contains(raw, "一次性登录链接") || strings.Contains(raw, "有效期至") {
		t.Fatalf("English magic link body contains Chinese copy in:\n%s", raw)
	}
}

func TestBuildApprovalNotificationMessageProducesProfessionalContent(t *testing.T) {
	st := model.SystemSettings{
		AppName:       "AI Shortlink",
		AppNameZH:     "AI短链平台",
		AppNameEN:     "AI Shortlink",
		BaseURL:       "https://s.example.com",
		DefaultLocale: "en-US",
	}
	n := approvalNotification{
		ResourceType:  "short_link",
		Title:         "Documentation portal",
		Code:          "docs",
		PublicURL:     "https://s.example.com/s/docs",
		Decision:      "approved",
		ReviewedAt:    time.Date(2026, 6, 30, 9, 15, 0, 0, time.UTC),
		RecipientName: "Taylor",
	}
	msg, envelopeFrom, err := buildApprovalNotificationMessage(st, "no-reply@mail.example.com", "user@example.com", n)
	if err != nil {
		t.Fatalf("buildApprovalNotificationMessage() error = %v", err)
	}
	if envelopeFrom != "no-reply@mail.example.com" {
		t.Fatalf("envelope from = %q", envelopeFrom)
	}
	parsed, err := mail.ReadMessage(bytes.NewReader(msg))
	if err != nil {
		t.Fatalf("ReadMessage() error = %v", err)
	}
	headers := parsed.Header
	for _, key := range []string{"From", "To", "Subject", "Date", "Message-Id", "Mime-Version", "Content-Type", "Content-Language", "Auto-Submitted"} {
		if headers.Get(key) == "" {
			t.Fatalf("missing %s header in:\n%s", key, msg)
		}
	}

	raw := string(msg)
	checks := []string{
		"Content-Language: en-US",
		"Content-Type: multipart/alternative;",
		"Content-Type: text/plain; charset=UTF-8",
		"Content-Type: text/html; charset=UTF-8",
		"Your submitted content has been approved",
		"Short link",
		"Documentation portal",
		"https://s.example.com/s/docs",
		"no attachments",
		"marketing",
	}
	for _, want := range checks {
		if !strings.Contains(raw, want) {
			t.Fatalf("approval message missing %q in:\n%s", want, raw)
		}
	}
	for _, risky := range []string{"!!!", "free", "winner", "act now"} {
		if strings.Contains(strings.ToLower(raw), risky) {
			t.Fatalf("approval message contains spammy token %q in:\n%s", risky, raw)
		}
	}
}

func TestBuildApprovalNotificationMessageIncludesRejectionReason(t *testing.T) {
	st := model.SystemSettings{
		AppName:       "AI Shortlink",
		AppNameEN:     "AI Shortlink",
		BaseURL:       "https://s.example.com",
		DefaultLocale: "en-US",
	}
	n := approvalNotification{
		ResourceType:  "live_qr",
		Title:         "Campaign QR",
		Code:          "campaign",
		PublicURL:     "https://s.example.com/q/campaign",
		Decision:      "rejected",
		ReviewNote:    "Please replace the expired destination URL.",
		ReviewedAt:    time.Date(2026, 7, 13, 8, 0, 0, 0, time.UTC),
		RecipientName: "Taylor",
	}
	msg, _, err := buildApprovalNotificationMessage(st, "no-reply@mail.example.com", "user@example.com", n)
	if err != nil {
		t.Fatalf("buildApprovalNotificationMessage() error = %v", err)
	}
	raw := string(msg)
	for _, want := range []string{
		"AI Shortlink rejection notification",
		"was not approved and is not publicly available",
		"Decision: Rejected",
		"Please replace the expired destination URL.",
		"https://s.example.com/admin",
	} {
		if !strings.Contains(raw, want) {
			t.Fatalf("rejection message missing %q in:\n%s", want, raw)
		}
	}
}

func TestReviewDecisionChangedForApprovalAndRejection(t *testing.T) {
	tests := []struct {
		before string
		after  string
		want   bool
	}{
		{before: "pending", after: "approved", want: true},
		{before: "pending", after: "rejected", want: true},
		{before: "approved", after: "rejected", want: true},
		{before: "rejected", after: "approved", want: true},
		{before: "approved", after: "approved", want: false},
		{before: "rejected", after: "rejected", want: false},
		{before: "approved", after: "pending", want: false},
	}
	for _, tt := range tests {
		if got := reviewDecisionChanged(tt.before, tt.after); got != tt.want {
			t.Errorf("reviewDecisionChanged(%q, %q) = %v, want %v", tt.before, tt.after, got, tt.want)
		}
	}
}

func TestMailDomainPrefersSenderDomain(t *testing.T) {
	st := model.SystemSettings{BaseURL: "http://localhost:8080"}
	if got := mailDomain(st, "no-reply@mail.example.com"); got != "mail.example.com" {
		t.Fatalf("mailDomain() = %q, want sender domain", got)
	}
}
