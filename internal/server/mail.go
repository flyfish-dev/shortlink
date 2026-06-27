package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"
)

func (s *Server) sendMagicLink(ctx context.Context, to, link string, expiresAt time.Time) error {
	st := s.settings(ctx)
	password := s.smtpPassword(ctx)
	if password == "" {
		return fmt.Errorf("SMTP password is not configured")
	}
	host := strings.TrimSpace(st.SMTPHost)
	addr := fmt.Sprintf("%s:%d", host, st.SMTPPort)
	from := strings.TrimSpace(st.SMTPFrom)
	username := strings.TrimSpace(st.SMTPUsername)
	if username == "" {
		username = from
	}
	appName := firstNonEmpty(st.AppName, st.AppNameZH, st.AppNameEN, "AI短链平台")
	subject := fmt.Sprintf("%s 登录链接", appName)
	body := fmt.Sprintf("请点击以下链接登录 %s：\n\n%s\n\n此链接将在 %s 过期，且只能使用一次。\n", appName, link, expiresAt.Format("2006-01-02 15:04:05"))
	if strings.HasPrefix(strings.ToLower(st.DefaultLocale), "en") {
		appName = firstNonEmpty(st.AppNameEN, st.AppName, "AI Shortlink")
		subject = fmt.Sprintf("%s login link", appName)
		body = fmt.Sprintf("Click the link below to sign in to %s:\n\n%s\n\nThis link expires at %s and can be used only once.\n", appName, link, expiresAt.Format("2006-01-02 15:04:05"))
	}
	msg := []byte("From: " + from + "\r\n" +
		"To: " + to + "\r\n" +
		"Subject: " + encodeMailSubject(subject) + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n\r\n" + body)
	security := strings.ToLower(strings.TrimSpace(st.SMTPSecurity))
	switch security {
	case "tls", "ssl":
		conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 10 * time.Second}, "tcp", addr, &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12})
		if err != nil {
			return err
		}
		defer conn.Close()
		c, err := smtp.NewClient(conn, host)
		if err != nil {
			return err
		}
		defer c.Close()
		return smtpSendWithClient(c, host, username, password, from, to, msg)
	case "starttls":
		c, err := smtp.Dial(addr)
		if err != nil {
			return err
		}
		defer c.Close()
		if err := c.StartTLS(&tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}); err != nil {
			return err
		}
		return smtpSendWithClient(c, host, username, password, from, to, msg)
	default:
		c, err := smtp.Dial(addr)
		if err != nil {
			return err
		}
		defer c.Close()
		return smtpSendWithClient(c, host, username, password, from, to, msg)
	}
}

func smtpSendWithClient(c *smtp.Client, host, username, password, from, to string, msg []byte) error {
	if username != "" && password != "" {
		if ok, _ := c.Extension("AUTH"); ok {
			if err := c.Auth(smtp.PlainAuth("", username, password, host)); err != nil {
				return err
			}
		}
	}
	if err := c.Mail(from); err != nil {
		return err
	}
	if err := c.Rcpt(to); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		_ = w.Close()
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}

func encodeMailSubject(s string) string {
	// UTF-8 subjects are accepted by most SMTP servers; keep it simple and standards-friendly enough for modern servers.
	return s
}
