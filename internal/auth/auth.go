package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	BrowserCookie     = "ais_browser"
	SessionCookie     = "ais_session"
	RecoveryKeyPrefix = "aisk_"
)

const tenYears = 10 * 365 * 24 * time.Hour

type Manager struct {
	secret       []byte
	sessionTTL   time.Duration
	cookieSecure bool
}

type Session struct {
	DeviceID  int64
	BrowserID string
	ExpiresAt time.Time
}

func NewManager(secret string, ttl time.Duration, secure bool) *Manager {
	if ttl <= 0 {
		ttl = tenYears
	}
	return &Manager{secret: []byte(secret), sessionTTL: ttl, cookieSecure: secure}
}

func RandomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func NewRecoveryKey() (string, error) {
	token, err := RandomToken(32)
	if err != nil {
		return "", err
	}
	return RecoveryKeyPrefix + token, nil
}

func NormalizeRecoveryKey(key string) string {
	key = strings.TrimSpace(key)
	key = strings.ReplaceAll(key, " ", "")
	key = strings.ReplaceAll(key, "\n", "")
	key = strings.ReplaceAll(key, "\r", "")
	key = strings.ReplaceAll(key, "\t", "")
	return key
}

func (m *Manager) Hash(value string) string {
	mac := hmac.New(sha256.New, m.secret)
	_, _ = mac.Write([]byte(value))
	return hex.EncodeToString(mac.Sum(nil))
}

func (m *Manager) RecoveryHash(key string) string {
	return m.Hash("recovery:" + NormalizeRecoveryKey(key))
}

func (m *Manager) Encrypt(value string) (string, error) {
	aead, err := m.aead()
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := aead.Seal(nonce, nonce, []byte(value), []byte("admin-account-recovery-key"))
	return base64.RawURLEncoding.EncodeToString(sealed), nil
}

func (m *Manager) Decrypt(blob string) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(blob))
	if err != nil {
		return "", err
	}
	aead, err := m.aead()
	if err != nil {
		return "", err
	}
	if len(raw) < aead.NonceSize() {
		return "", errors.New("ciphertext too short")
	}
	nonce := raw[:aead.NonceSize()]
	ciphertext := raw[aead.NonceSize():]
	plain, err := aead.Open(nil, nonce, ciphertext, []byte("admin-account-recovery-key"))
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func (m *Manager) aead() (cipher.AEAD, error) {
	key := sha256.Sum256(m.secret)
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func (m *Manager) EnsureBrowserID(w http.ResponseWriter, r *http.Request) (string, error) {
	if c, err := r.Cookie(BrowserCookie); err == nil && len(c.Value) >= 24 {
		return c.Value, nil
	}
	token, err := RandomToken(32)
	if err != nil {
		return "", err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     BrowserCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   m.cookieSecure,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(tenYears),
		MaxAge:   int(tenYears.Seconds()),
	})
	return token, nil
}

func (m *Manager) BrowserID(r *http.Request) string {
	c, err := r.Cookie(BrowserCookie)
	if err != nil {
		return ""
	}
	return c.Value
}

func (m *Manager) SetSession(w http.ResponseWriter, deviceID int64, browserID string) {
	exp := time.Now().Add(m.sessionTTL)
	body := fmt.Sprintf("%d|%s|%d", deviceID, browserID, exp.Unix())
	sig := m.sign(body)
	value := base64.RawURLEncoding.EncodeToString([]byte(body + "|" + sig))
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookie,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   m.cookieSecure,
		SameSite: http.SameSiteLaxMode,
		Expires:  exp,
		MaxAge:   int(m.sessionTTL.Seconds()),
	})
}

func (m *Manager) Clear(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: SessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: m.cookieSecure, SameSite: http.SameSiteLaxMode})
}

func (m *Manager) ReadSession(r *http.Request) (*Session, error) {
	c, err := r.Cookie(SessionCookie)
	if err != nil {
		return nil, err
	}
	raw, err := base64.RawURLEncoding.DecodeString(c.Value)
	if err != nil {
		return nil, err
	}
	parts := strings.Split(string(raw), "|")
	if len(parts) == 4 {
		return m.readV2Session(parts)
	}
	if len(parts) == 5 {
		return m.readLegacySession(parts)
	}
	return nil, errors.New("bad session")
}

func (m *Manager) readV2Session(parts []string) (*Session, error) {
	body := strings.Join(parts[:3], "|")
	if !hmac.Equal([]byte(parts[3]), []byte(m.sign(body))) {
		return nil, errors.New("bad signature")
	}
	deviceID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return nil, err
	}
	expUnix, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return nil, err
	}
	exp := time.Unix(expUnix, 0)
	if time.Now().After(exp) {
		return nil, errors.New("session expired")
	}
	return &Session{DeviceID: deviceID, BrowserID: parts[1], ExpiresAt: exp}, nil
}

func (m *Manager) readLegacySession(parts []string) (*Session, error) {
	body := strings.Join(parts[:4], "|")
	if !hmac.Equal([]byte(parts[4]), []byte(m.sign(body))) {
		return nil, errors.New("bad signature")
	}
	deviceID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return nil, err
	}
	expUnix, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		return nil, err
	}
	exp := time.Unix(expUnix, 0)
	if time.Now().After(exp) {
		return nil, errors.New("session expired")
	}
	return &Session{DeviceID: deviceID, BrowserID: parts[1], ExpiresAt: exp}, nil
}

func (m *Manager) sign(body string) string {
	mac := hmac.New(sha256.New, m.secret)
	_, _ = mac.Write([]byte("session:" + body))
	return hex.EncodeToString(mac.Sum(nil))
}
