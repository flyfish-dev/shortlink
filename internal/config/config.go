package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const RuntimeConfigFile = "app-config.json"

type Config struct {
	AppName            string
	BaseURL            string
	Addr               string
	DatabaseMode       string
	DSN                string
	SQLitePath         string
	AppSecret          string
	DataDir            string
	AutoMigrate        bool
	TrustProxy         bool
	CookieSecure       bool
	SessionTTL         time.Duration
	UploadMaxBytes     int64
	GitHubClientID     string
	GitHubClientSecret string
}

type RuntimeConfig struct {
	DatabaseMode string `json:"database_mode"`
	DSN          string `json:"dsn"`
	SQLitePath   string `json:"sqlite_path"`
	AppSecret    string `json:"app_secret"`
	TrustProxy   bool   `json:"trust_proxy"`
	CookieSecure bool   `json:"cookie_secure"`
}

func Load() Config {
	dataDir := getenv("DATA_DIR", "./data")
	rc, _ := LoadRuntime(dataDir)
	c := Config{
		AppName:            getenv("APP_NAME", "AI短链平台"),
		BaseURL:            strings.TrimRight(os.Getenv("APP_BASE_URL"), "/"),
		Addr:               defaultAddr(),
		DatabaseMode:       firstNonEmpty(os.Getenv("DATABASE_MODE"), rc.DatabaseMode, "embedded"),
		DSN:                firstNonEmpty(os.Getenv("DATABASE_DSN"), rc.DSN, "shortlink:shortlink@tcp(127.0.0.1:3306)/ai_shortlink?charset=utf8mb4&parseTime=true&loc=Local&multiStatements=true"),
		SQLitePath:         firstNonEmpty(os.Getenv("SQLITE_PATH"), rc.SQLitePath, filepath.Join(dataDir, "ai-shortlink.db")),
		AppSecret:          firstNonEmpty(os.Getenv("APP_SECRET"), rc.AppSecret),
		DataDir:            dataDir,
		AutoMigrate:        getenvBool("AUTO_MIGRATE", true),
		TrustProxy:         getenvBoolWithRuntime("TRUST_PROXY", rc.TrustProxy, false),
		CookieSecure:       getenvBoolWithRuntime("COOKIE_SECURE", rc.CookieSecure, false),
		SessionTTL:         time.Duration(getenvInt("SESSION_TTL_HOURS", 24*365*10)) * time.Hour,
		UploadMaxBytes:     int64(getenvInt("UPLOAD_MAX_MB", 8)) * 1024 * 1024,
		GitHubClientID:     strings.TrimSpace(os.Getenv("GITHUB_CLIENT_ID")),
		GitHubClientSecret: strings.TrimSpace(os.Getenv("GITHUB_CLIENT_SECRET")),
	}
	c.DatabaseMode = NormalizeDatabaseMode(c.DatabaseMode)
	if c.AppSecret == "" {
		c.AppSecret = randomSecret()
		fmt.Println("[WARN] APP_SECRET was empty; generated and saved a local secret in data/app-config.json.")
		_ = SaveRuntime(dataDir, RuntimeConfig{DatabaseMode: c.DatabaseMode, DSN: c.DSN, SQLitePath: c.SQLitePath, AppSecret: c.AppSecret, TrustProxy: c.TrustProxy, CookieSecure: c.CookieSecure})
	}
	return c
}

func NormalizeDatabaseMode(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	switch v {
	case "mysql", "mariadb":
		return "mysql"
	case "sqlite", "embedded", "embed", "local", "file":
		return "embedded"
	default:
		return "embedded"
	}
}

func RuntimePath(dataDir string) string { return filepath.Join(dataDir, RuntimeConfigFile) }

func LoadRuntime(dataDir string) (RuntimeConfig, error) {
	var rc RuntimeConfig
	b, err := os.ReadFile(RuntimePath(dataDir))
	if err != nil {
		return rc, err
	}
	if err := json.Unmarshal(b, &rc); err != nil {
		return rc, err
	}
	rc.DatabaseMode = NormalizeDatabaseMode(rc.DatabaseMode)
	return rc, nil
}

func SaveRuntime(dataDir string, rc RuntimeConfig) error {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return err
	}
	rc.DatabaseMode = NormalizeDatabaseMode(rc.DatabaseMode)
	b, err := json.MarshalIndent(rc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(RuntimePath(dataDir), append(b, '\n'), 0600)
}

func (c Config) RuntimeConfig() RuntimeConfig {
	return RuntimeConfig{DatabaseMode: c.DatabaseMode, DSN: c.DSN, SQLitePath: c.SQLitePath, AppSecret: c.AppSecret, TrustProxy: c.TrustProxy, CookieSecure: c.CookieSecure}
}

func (c Config) WithRuntime(rc RuntimeConfig) Config {
	c.DatabaseMode = NormalizeDatabaseMode(rc.DatabaseMode)
	if strings.TrimSpace(rc.DSN) != "" {
		c.DSN = rc.DSN
	}
	if strings.TrimSpace(rc.SQLitePath) != "" {
		c.SQLitePath = rc.SQLitePath
	}
	if strings.TrimSpace(rc.AppSecret) != "" {
		c.AppSecret = rc.AppSecret
	}
	c.TrustProxy = rc.TrustProxy
	c.CookieSecure = rc.CookieSecure
	return c
}

func getenv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func getenvBoolWithRuntime(key string, runtime, fallback bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		if runtime {
			return true
		}
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

func defaultAddr() string {
	if v := strings.TrimSpace(os.Getenv("APP_ADDR")); v != "" {
		return v
	}

	if p := strings.TrimSpace(os.Getenv("PORT")); p != "" {
		if strings.HasPrefix(p, ":") {
			return p
		}
		return ":" + p
	}

	return ":8080"
}

func getenvBool(key string, fallback bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

func getenvInt(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func randomSecret() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "dev-secret-change-me"
	}
	return hex.EncodeToString(b)
}
