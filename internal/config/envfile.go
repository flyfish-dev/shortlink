package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

func init() { loadEnvFiles() }

func loadEnvFiles() {
	paths := []string{strings.TrimSpace(os.Getenv("SHORTLINK_CONFIG")), "shortlink.env", ".env"}
	seen := map[string]bool{}
	for _, path := range paths {
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true
		_ = loadEnvFile(path)
	}
}

func loadEnvFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" || strings.ContainsAny(key, " \t") {
			continue
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		_ = os.Setenv(key, parseEnvValue(val))
	}
	return s.Err()
}

func parseEnvValue(v string) string {
	v = strings.TrimSpace(v)
	if len(v) >= 2 {
		quote := v[0]
		if (quote == '\'' || quote == '"') && v[len(v)-1] == quote {
			out, err := strconv.Unquote(v)
			if err == nil {
				return out
			}
			return v[1 : len(v)-1]
		}
	}
	if idx := strings.Index(v, " #"); idx >= 0 {
		v = strings.TrimSpace(v[:idx])
	}
	return v
}
