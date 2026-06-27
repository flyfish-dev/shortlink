package util

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"
)

var codeRe = regexp.MustCompile(`^[A-Za-z0-9_-]{3,64}$`)

var reservedCodes = map[string]bool{
	"admin": true, "api": true, "login": true, "logout": true, "auth": true,
	"assets": true, "static": true, "uploads": true, "q": true, "s": true, "qr": true,
	"healthz": true, "favicon.ico": true, "robots.txt": true,
}

func ValidateCode(code string) error {
	code = strings.TrimSpace(code)
	if !codeRe.MatchString(code) {
		return errors.New("短码只能包含 3-64 位字母、数字、下划线或中划线")
	}
	if reservedCodes[strings.ToLower(code)] {
		return errors.New("该短码为系统保留词")
	}
	return nil
}

func RandomCode(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	s := base64.RawURLEncoding.EncodeToString(b)
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, "_", "")
	if len(s) < n {
		return RandomCode(n)
	}
	return s[:n], nil
}

func ClientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			if ip := strings.TrimSpace(parts[0]); net.ParseIP(ip) != nil {
				return ip
			}
		}
		if xrip := strings.TrimSpace(r.Header.Get("X-Real-IP")); xrip != "" && net.ParseIP(xrip) != nil {
			return xrip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		if net.ParseIP(r.RemoteAddr) != nil {
			return r.RemoteAddr
		}
		return ""
	}
	return host
}

func NormalizeIPForLogin(ip string) string {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return ip
	}
	if v4 := parsed.To4(); v4 != nil {
		// Bind to IPv4 /24 to avoid frequent mobile-network last-octet changes.
		return net.IPv4(v4[0], v4[1], v4[2], 0).String() + "/24"
	}
	// Bind to IPv6 /64.
	v16 := parsed.To16()
	if v16 == nil {
		return ip
	}
	masked := make(net.IP, len(v16))
	copy(masked, v16)
	for i := 8; i < 16; i++ {
		masked[i] = 0
	}
	return masked.String() + "/64"
}

func PublicBaseURL(r *http.Request, configured string, trustProxy bool) string {
	if configured != "" {
		return strings.TrimRight(configured, "/")
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if trustProxy {
		if proto := r.Header.Get("X-Forwarded-Proto"); proto == "http" || proto == "https" {
			scheme = proto
		}
	}
	return scheme + "://" + r.Host
}

func ParseAPITime(s string) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	layouts := []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04", "2006-01-02"}
	for _, layout := range layouts {
		t, err := time.ParseInLocation(layout, s, time.Local)
		if err == nil {
			return &t, nil
		}
	}
	return nil, errors.New("时间格式应为 RFC3339、YYYY-MM-DD HH:mm:ss 或 YYYY-MM-DD")
}

func StringPtrValue(p *time.Time) string {
	if p == nil {
		return ""
	}
	return p.Format("2006-01-02 15:04:05")
}

func DetectClient(ua string) (device, browser, os string) {
	u := strings.ToLower(ua)
	switch {
	case strings.Contains(u, "micromessenger"):
		browser = "WeChat"
	case strings.Contains(u, "edg/") || strings.Contains(u, "edge/"):
		browser = "Edge"
	case strings.Contains(u, "chrome/") || strings.Contains(u, "crios/"):
		browser = "Chrome"
	case strings.Contains(u, "safari/") && !strings.Contains(u, "chrome/"):
		browser = "Safari"
	case strings.Contains(u, "firefox/"):
		browser = "Firefox"
	default:
		browser = "Other"
	}
	switch {
	case strings.Contains(u, "iphone") || strings.Contains(u, "android") && strings.Contains(u, "mobile"):
		device = "Mobile"
	case strings.Contains(u, "ipad") || strings.Contains(u, "tablet"):
		device = "Tablet"
	case strings.Contains(u, "bot") || strings.Contains(u, "spider") || strings.Contains(u, "crawler"):
		device = "Bot"
	default:
		device = "Desktop"
	}
	switch {
	case strings.Contains(u, "windows"):
		os = "Windows"
	case strings.Contains(u, "mac os") || strings.Contains(u, "macintosh"):
		os = "macOS"
	case strings.Contains(u, "iphone") || strings.Contains(u, "ipad"):
		os = "iOS"
	case strings.Contains(u, "android"):
		os = "Android"
	case strings.Contains(u, "linux"):
		os = "Linux"
	default:
		os = "Other"
	}
	return
}

func CleanURL(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	lower := strings.ToLower(s)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return s
	}
	return "https://" + s
}

func Truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
