package server

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"html"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/url"
	"strings"
	"time"

	"ai-shortlink/internal/model"
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
	msg, envelopeFrom, err := buildMagicLinkMessage(st, from, to, link, expiresAt)
	if err != nil {
		return err
	}
	helloName := smtpHelloName(st, envelopeFrom)
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
		return smtpSendWithClient(c, helloName, false, host, username, password, envelopeFrom, to, msg)
	case "starttls":
		c, err := smtp.Dial(addr)
		if err != nil {
			return err
		}
		defer c.Close()
		if err := c.Hello(helloName); err != nil {
			return err
		}
		if err := c.StartTLS(&tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}); err != nil {
			return err
		}
		return smtpSendWithClient(c, helloName, true, host, username, password, envelopeFrom, to, msg)
	default:
		c, err := smtp.Dial(addr)
		if err != nil {
			return err
		}
		defer c.Close()
		return smtpSendWithClient(c, helloName, false, host, username, password, envelopeFrom, to, msg)
	}
}

func buildMagicLinkMessage(st model.SystemSettings, from, to, link string, expiresAt time.Time) ([]byte, string, error) {
	fromAddr, err := parseMailAddress(from)
	if err != nil {
		return nil, "", fmt.Errorf("SMTP from email is invalid: %w", err)
	}
	toAddr, err := parseMailAddress(to)
	if err != nil {
		return nil, "", fmt.Errorf("recipient email is invalid: %w", err)
	}
	lang := "zh-CN"
	appName := firstNonEmpty(st.AppName, st.AppNameZH, st.AppNameEN, "AI短链平台")
	subject := fmt.Sprintf("%s 登录链接", appName)
	expText := expiresAt.Format("2006-01-02 15:04:05")
	textBody := fmt.Sprintf("%s 的一次性登录链接：\r\n\r\n%s\r\n\r\n有效期至：%s\r\n此链接只能使用一次。如果不是你发起的请求，可以忽略这封邮件。\r\n", appName, link, expText)
	htmlBody := magicLinkHTML(appName, subject, link, expText, "打开登录链接", "此链接只能使用一次。如果不是你发起的请求，可以忽略这封邮件。")
	if strings.HasPrefix(strings.ToLower(st.DefaultLocale), "en") {
		lang = "en-US"
		appName = firstNonEmpty(st.AppNameEN, st.AppName, "AI Shortlink")
		subject = fmt.Sprintf("%s login link", appName)
		expText = expiresAt.Format(time.RFC1123Z)
		textBody = fmt.Sprintf("Your one-time login link for %s:\r\n\r\n%s\r\n\r\nExpires at: %s\r\nThis link can be used only once. If you did not request it, you can ignore this email.\r\n", appName, link, expText)
		htmlBody = magicLinkHTML(appName, subject, link, expText, "Open login link", "This link can be used only once. If you did not request it, you can ignore this email.")
	}

	displayFrom := mail.Address{Name: appName, Address: fromAddr.Address}
	boundary := "asl-" + randomHex(12)
	var b strings.Builder
	writeMailHeader(&b, "From", displayFrom.String())
	writeMailHeader(&b, "To", toAddr.String())
	writeMailHeader(&b, "Subject", encodeMailSubject(subject))
	writeMailHeader(&b, "Date", time.Now().Format(time.RFC1123Z))
	writeMailHeader(&b, "Message-ID", messageID(st, fromAddr.Address))
	writeMailHeader(&b, "MIME-Version", "1.0")
	writeMailHeader(&b, "Content-Language", lang)
	writeMailHeader(&b, "Auto-Submitted", "auto-generated")
	writeMailHeader(&b, "X-Auto-Response-Suppress", "All")
	writeMailHeader(&b, "Content-Type", fmt.Sprintf(`multipart/alternative; boundary="%s"`, boundary))
	b.WriteString("\r\n")

	writeMIMEPart(&b, boundary, "text/plain; charset=UTF-8", textBody)
	writeMIMEPart(&b, boundary, "text/html; charset=UTF-8", htmlBody)
	b.WriteString("--" + boundary + "--\r\n")
	return []byte(b.String()), fromAddr.Address, nil
}

func parseMailAddress(addr string) (*mail.Address, error) {
	parsed, err := mail.ParseAddress(strings.TrimSpace(addr))
	if err == nil {
		return parsed, nil
	}
	if strings.Contains(addr, "@") && !strings.ContainsAny(addr, " <>\r\n") {
		return &mail.Address{Address: strings.TrimSpace(addr)}, nil
	}
	return nil, err
}

func writeMailHeader(b *strings.Builder, key, value string) {
	value = strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(value), "\r", ""), "\n", "")
	b.WriteString(key + ": " + value + "\r\n")
}

func writeMIMEPart(b *strings.Builder, boundary, contentType, body string) {
	b.WriteString("--" + boundary + "\r\n")
	writeMailHeader(b, "Content-Type", contentType)
	writeMailHeader(b, "Content-Transfer-Encoding", "quoted-printable")
	b.WriteString("\r\n")
	var encoded bytes.Buffer
	qp := quotedprintable.NewWriter(&encoded)
	_, _ = qp.Write([]byte(body))
	_ = qp.Close()
	b.Write(encoded.Bytes())
	b.WriteString("\r\n")
}

func magicLinkHTML(appName, title, link, expiresAt, buttonText, footnote string) string {
	safeApp := html.EscapeString(appName)
	safeTitle := html.EscapeString(title)
	safeLink := html.EscapeString(link)
	safeExpires := html.EscapeString(expiresAt)
	safeButton := html.EscapeString(buttonText)
	safeFootnote := html.EscapeString(footnote)
	return `<!doctype html>
<html>
<body style="margin:0;padding:0;background:#f6f7fb;color:#111827;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Arial,sans-serif;">
  <div style="max-width:560px;margin:0 auto;padding:32px 20px;">
    <div style="background:#ffffff;border:1px solid #e5e7eb;border-radius:8px;padding:28px;">
      <div style="font-size:13px;font-weight:700;color:#2563eb;margin-bottom:12px;">` + safeApp + `</div>
      <h1 style="font-size:22px;line-height:1.3;margin:0 0 14px;color:#111827;">` + safeTitle + `</h1>
      <p style="font-size:15px;line-height:1.7;margin:0 0 22px;color:#4b5563;">` + safeFootnote + `</p>
      <p style="margin:0 0 22px;"><a href="` + safeLink + `" style="display:inline-block;background:#111827;color:#ffffff;text-decoration:none;border-radius:6px;padding:12px 18px;font-weight:700;">` + safeButton + `</a></p>
      <p style="font-size:13px;line-height:1.7;margin:0 0 8px;color:#6b7280;">Expires: ` + safeExpires + `</p>
      <p style="font-size:12px;line-height:1.7;margin:0;color:#6b7280;word-break:break-all;">` + safeLink + `</p>
    </div>
  </div>
</body>
</html>`
}

func messageID(st model.SystemSettings, from string) string {
	domain := mailDomain(st, from)
	return fmt.Sprintf("<%d.%s@%s>", time.Now().UnixNano(), randomHex(8), domain)
}

func smtpHelloName(st model.SystemSettings, from string) string {
	return mailDomain(st, from)
}

func mailDomain(st model.SystemSettings, from string) string {
	if i := strings.LastIndex(from, "@"); i >= 0 {
		if host := cleanMailHost(from[i+1:]); host != "" {
			return host
		}
	}
	if u, err := url.Parse(strings.TrimSpace(st.BaseURL)); err == nil && u.Hostname() != "" {
		if host := cleanMailHost(u.Hostname()); host != "" {
			return host
		}
	}
	return "localhost"
}

func cleanMailHost(host string) string {
	host = strings.Trim(strings.TrimSpace(strings.ToLower(host)), ".")
	if host == "" || strings.ContainsAny(host, " \t\r\n") {
		return ""
	}
	if ip := net.ParseIP(host); ip != nil {
		return ""
	}
	if host == "localhost" || !strings.Contains(host, ".") {
		return ""
	}
	return host
}

func randomHex(n int) string {
	if n <= 0 {
		n = 8
	}
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

func smtpSendWithClient(c *smtp.Client, helloName string, helloDone bool, host, username, password, from, to string, msg []byte) error {
	if !helloDone {
		if err := c.Hello(helloName); err != nil {
			return err
		}
	}
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
	return mime.QEncoding.Encode("UTF-8", s)
}
