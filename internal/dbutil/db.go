package dbutil

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "ai-shortlink/internal/mysqlmini"
	_ "ai-shortlink/internal/sqlitecgo"
)

//go:embed migrations/mysql/*.sql migrations/sqlite/*.sql
var migrationFS embed.FS

func Open(ctx context.Context, mode, dsn, sqlitePath string) (*sql.DB, error) {
	mode = normalizeMode(mode)
	var driver, source string
	if mode == "mysql" {
		driver, source = "mysqlmini", dsn
	} else {
		if strings.TrimSpace(sqlitePath) == "" {
			sqlitePath = "./data/ai-shortlink.db"
		}
		if err := os.MkdirAll(filepath.Dir(sqlitePath), 0755); err != nil {
			return nil, err
		}
		driver, source = "sqlitecgo", sqlitePath
	}
	db, err := sql.Open(driver, source)
	if err != nil {
		return nil, err
	}
	if mode == "embedded" {
		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)
	} else {
		db.SetMaxOpenConns(25)
		db.SetMaxIdleConns(10)
		db.SetConnMaxLifetime(30 * time.Minute)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func Migrate(ctx context.Context, db *sql.DB, mode string) error {
	mode = normalizeMode(mode)
	dir := "migrations/mysql"
	if mode == "embedded" {
		dir = "migrations/sqlite"
	}
	entries, err := migrationFS.ReadDir(dir)
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	ensureSchema := `CREATE TABLE IF NOT EXISTS schema_migrations (
        version VARCHAR(64) PRIMARY KEY,
        applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`
	if mode == "embedded" {
		ensureSchema = `CREATE TABLE IF NOT EXISTS schema_migrations (
            version TEXT PRIMARY KEY,
            applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
        )`
	}
	if _, err := db.ExecContext(ctx, ensureSchema); err != nil {
		return fmt.Errorf("ensure schema_migrations: %w", err)
	}

	for _, name := range names {
		var exists int
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations WHERE version = ?", name).Scan(&exists); err != nil {
			return err
		}
		if exists > 0 {
			continue
		}
		b, err := migrationFS.ReadFile(dir + "/" + name)
		if err != nil {
			return err
		}
		stmts := splitSQLStatements(string(b))
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		for _, stmt := range stmts {
			if strings.TrimSpace(stmt) == "" {
				continue
			}
			if _, err := tx.ExecContext(ctx, stmt); err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("migration %s failed near %q: %w", name, trimForLog(stmt), err)
			}
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations(version) VALUES (?)", name); err != nil {
			_ = tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		log.Printf("applied %s migration %s", mode, name)
	}
	return nil
}

func normalizeMode(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	if v == "mysql" || v == "mariadb" {
		return "mysql"
	}
	return "embedded"
}

func splitSQLStatements(sqlText string) []string {
	var stmts []string
	var b strings.Builder
	inSingle, inDouble, inLineComment, inBlockComment := false, false, false, false
	prev := rune(0)
	for _, r := range sqlText {
		if inLineComment {
			b.WriteRune(r)
			if r == '\n' {
				inLineComment = false
			}
			prev = r
			continue
		}
		if inBlockComment {
			b.WriteRune(r)
			if prev == '*' && r == '/' {
				inBlockComment = false
			}
			prev = r
			continue
		}
		if !inSingle && !inDouble {
			if prev == '-' && r == '-' {
				inLineComment = true
				b.WriteRune(r)
				prev = r
				continue
			}
			if prev == '/' && r == '*' {
				inBlockComment = true
				b.WriteRune(r)
				prev = r
				continue
			}
		}
		switch r {
		case '\'':
			if !inDouble && prev != '\\' {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case ';':
			if !inSingle && !inDouble {
				stmts = append(stmts, b.String())
				b.Reset()
				prev = r
				continue
			}
		}
		b.WriteRune(r)
		prev = r
	}
	if strings.TrimSpace(b.String()) != "" {
		stmts = append(stmts, b.String())
	}
	return stmts
}

func trimForLog(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 180 {
		return s[:180] + "..."
	}
	return s
}
