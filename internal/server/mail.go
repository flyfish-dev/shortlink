package server

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/url"
	"strings"
	"time"

	"ai-shortlink/internal/model"
	"ai-shortlink/internal/server/mailtpl"
)

func (s *Server) sendMagicLink(ctx context.Context, to, link string, expiresAt time.Time) error {
	return s.sendTransactionalMail(ctx, to, func(st model.SystemSettings, from, to string) ([]byte, string, error) {
		return buildMagicLinkMessage(st, from, to, link, expiresAt)
	})
}

func (s *Server) sendApprovalNotificationMail(ctx context.Context, to string, n approvalNotification) error {
	return s.sendTransactionalMail(ctx, to, func(st model.SystemSettings, from, to string) ([]byte, string, error) {
		return buildApprovalNotificationMessage(st, from, to, n)
	})
}

func (s *Server) sendTransactionalMail(ctx context.Context, to string, build func(model.SystemSettings, string, string) ([]byte, string, error)) error {
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
	msg, envelopeFrom, err := build(st, from, to)
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

type transactionalEmail struct {
	AppName string
	Lang    string
	Subject string
	Text    string
	HTML    string
}

func buildMagicLinkMessage(st model.SystemSettings, from, to, link string, expiresAt time.Time) ([]byte, string, error) {
	lang := "zh-CN"
	appName := firstNonEmpty(st.AppName, st.AppNameZH, st.AppNameEN, "AI短链平台")
	subject := fmt.Sprintf("%s 登录链接", appName)
	expText := expiresAt.Format("2006-01-02 15:04:05")
	data := mailtpl.MagicLinkData{
		AppName:      appName,
		Title:        subject,
		Intro:        fmt.Sprintf("%s 的一次性登录链接：", appName),
		Link:         link,
		ExpiresAt:    expText,
		ExpiresLabel: "有效期至",
		ButtonText:   "打开登录链接",
		Footnote:     "此链接只能使用一次。如果不是你发起的请求，可以忽略这封邮件。",
	}
	if strings.HasPrefix(strings.ToLower(st.DefaultLocale), "en") {
		lang = "en-US"
		appName = firstNonEmpty(st.AppNameEN, st.AppName, "AI Shortlink")
		subject = fmt.Sprintf("%s login link", appName)
		expText = expiresAt.Format(time.RFC1123Z)
		data = mailtpl.MagicLinkData{
			AppName:      appName,
			Title:        subject,
			Intro:        fmt.Sprintf("Your one-time login link for %s:", appName),
			Link:         link,
			ExpiresAt:    expText,
			ExpiresLabel: "Expires at",
			ButtonText:   "Open login link",
			Footnote:     "This link can be used only once. If you did not request it, you can ignore this email.",
		}
	}
	return buildTransactionalMessage(st, from, to, transactionalEmail{AppName: appName, Lang: lang, Subject: subject, Text: mailtpl.MagicLinkText(data), HTML: mailtpl.MagicLinkHTML(data)})
}

func buildApprovalNotificationMessage(st model.SystemSettings, from, to string, n approvalNotification) ([]byte, string, error) {
	isEN := strings.HasPrefix(strings.ToLower(st.DefaultLocale), "en")
	appName := firstNonEmpty(st.AppName, st.AppNameZH, st.AppNameEN, "AI短链平台")
	lang := "zh-CN"
	resourceType := approvalResourceLabel(n.ResourceType, false)
	title := firstNonEmpty(n.Title, n.Code, resourceType)
	greeting := firstNonEmpty(n.RecipientName, "你好")
	subject := fmt.Sprintf("%s 审核通过通知", appName)
	statusLine := "你提交的内容已通过审核，现在可以正常访问。"
	buttonText := "查看内容"
	footer := "这是一封系统审核通知邮件，不包含附件或营销内容。如果你没有提交该内容，请联系系统管理员。"
	approvedAt := approvalMailTime(n.ApprovedAt, false)
	nextStep := "你可以在管理后台继续查看访问数据和二维码下载。"
	rows := []mailtpl.InfoRow{
		{Label: "类型", Value: resourceType},
		{Label: "名称", Value: title},
		{Label: "公开地址", Value: n.PublicURL},
		{Label: "审核时间", Value: approvedAt},
	}
	if n.ParentTitle != "" {
		rows = []mailtpl.InfoRow{
			{Label: "类型", Value: resourceType},
			{Label: "所属活码", Value: n.ParentTitle},
			{Label: "名称", Value: title},
			{Label: "活码入口", Value: n.PublicURL},
			{Label: "审核时间", Value: approvedAt},
		}
	}
	data := mailtpl.ApprovalData{AppName: appName, Title: subject, Greeting: greeting, Intro: statusLine, NextStep: nextStep, ButtonText: buttonText, ButtonURL: n.PublicURL, Rows: rows, Footnote: footer}
	if isEN {
		lang = "en-US"
		appName = firstNonEmpty(st.AppNameEN, st.AppName, "AI Shortlink")
		resourceType = approvalResourceLabel(n.ResourceType, true)
		title = firstNonEmpty(n.Title, n.Code, resourceType)
		greeting = firstNonEmpty(n.RecipientName, "Hello")
		subject = fmt.Sprintf("%s approval notification", appName)
		statusLine = "Your submitted content has been approved and is now available."
		buttonText = "View content"
		footer = "This is a system review notification. It has no attachments and no marketing content. If you did not submit this content, contact your administrator."
		approvedAt = approvalMailTime(n.ApprovedAt, true)
		nextStep = "You can continue to review analytics and QR downloads in the admin console."
		rows = []mailtpl.InfoRow{
			{Label: "Type", Value: resourceType},
			{Label: "Name", Value: title},
			{Label: "Public URL", Value: n.PublicURL},
			{Label: "Approved at", Value: approvedAt},
		}
		if n.ParentTitle != "" {
			rows = []mailtpl.InfoRow{
				{Label: "Type", Value: resourceType},
				{Label: "Live QR", Value: n.ParentTitle},
				{Label: "Name", Value: title},
				{Label: "Entry URL", Value: n.PublicURL},
				{Label: "Approved at", Value: approvedAt},
			}
		}
		data = mailtpl.ApprovalData{AppName: appName, Title: subject, Greeting: greeting, Intro: statusLine, NextStep: nextStep, ButtonText: buttonText, ButtonURL: n.PublicURL, Rows: rows, Footnote: footer}
	}
	return buildTransactionalMessage(st, from, to, transactionalEmail{AppName: appName, Lang: lang, Subject: subject, Text: mailtpl.ApprovalText(data), HTML: mailtpl.ApprovalHTML(data)})
}

func buildTransactionalMessage(st model.SystemSettings, from, to string, email transactionalEmail) ([]byte, string, error) {
	fromAddr, err := parseMailAddress(from)
	if err != nil {
		return nil, "", fmt.Errorf("SMTP from email is invalid: %w", err)
	}
	toAddr, err := parseMailAddress(to)
	if err != nil {
		return nil, "", fmt.Errorf("recipient email is invalid: %w", err)
	}

	displayFrom := mail.Address{Name: email.AppName, Address: fromAddr.Address}
	boundary := "asl-" + randomHex(12)
	var b strings.Builder
	writeMailHeader(&b, "From", displayFrom.String())
	writeMailHeader(&b, "To", toAddr.String())
	writeMailHeader(&b, "Subject", encodeMailSubject(email.Subject))
	writeMailHeader(&b, "Date", time.Now().Format(time.RFC1123Z))
	writeMailHeader(&b, "Message-ID", messageID(st, fromAddr.Address))
	writeMailHeader(&b, "MIME-Version", "1.0")
	writeMailHeader(&b, "Content-Language", firstNonEmpty(email.Lang, "zh-CN"))
	writeMailHeader(&b, "Auto-Submitted", "auto-generated")
	writeMailHeader(&b, "X-Auto-Response-Suppress", "All")
	writeMailHeader(&b, "Content-Type", fmt.Sprintf(`multipart/alternative; boundary="%s"`, boundary))
	b.WriteString("\r\n")

	writeMIMEPart(&b, boundary, "text/plain; charset=UTF-8", email.Text)
	writeMIMEPart(&b, boundary, "text/html; charset=UTF-8", email.HTML)
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
	_, _ = qp.Write([]byte(normalizeMailBody(body)))
	_ = qp.Close()
	b.Write(encoded.Bytes())
	b.WriteString("\r\n")
}

func normalizeMailBody(body string) string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	body = strings.ReplaceAll(body, "\r", "\n")
	return strings.ReplaceAll(body, "\n", "\r\n")
}

func approvalResourceLabel(resourceType string, en bool) string {
	switch resourceType {
	case "short_link":
		if en {
			return "Short link"
		}
		return "短链"
	case "live_qr":
		if en {
			return "Live QR"
		}
		return "活码"
	case "live_qr_item":
		if en {
			return "Live QR item"
		}
		return "活码二维码"
	default:
		if en {
			return "Content"
		}
		return "内容"
	}
}

func approvalMailTime(t time.Time, en bool) string {
	if t.IsZero() {
		t = time.Now()
	}
	if en {
		return t.Format(time.RFC1123Z)
	}
	return t.Format("2006-01-02 15:04:05 MST")
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
