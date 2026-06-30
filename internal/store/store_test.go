package store_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"ai-shortlink/internal/dbutil"
	"ai-shortlink/internal/store"
)

func TestFindActiveMagicLoginTokenByEmail(t *testing.T) {
	ctx := context.Background()
	db, err := dbutil.Open(ctx, "embedded", "", filepath.Join(t.TempDir(), "shortlink.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	if err := dbutil.Migrate(ctx, db, "embedded"); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	st := store.New(db, "embedded")
	acct, err := st.CreateAdminAccount(ctx, "Admin@Example.com", "Admin", "recovery-hash", "recovery-cipher")
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	expired, err := st.CreateMagicLoginToken(ctx, acct.ID, "admin@example.com", "expired-token", time.Now().Add(-time.Minute), "127.0.0.1")
	if err != nil {
		t.Fatalf("create expired token: %v", err)
	}
	if _, err := st.FindActiveMagicLoginTokenByEmail(ctx, "ADMIN@example.com", time.Now()); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expired token lookup error = %v, want ErrNotFound", err)
	}

	active, err := st.CreateMagicLoginToken(ctx, acct.ID, "Admin@Example.com", "active-token", time.Now().Add(15*time.Minute), "127.0.0.1")
	if err != nil {
		t.Fatalf("create active token: %v", err)
	}
	got, err := st.FindActiveMagicLoginTokenByEmail(ctx, "admin@example.com", time.Now())
	if err != nil {
		t.Fatalf("find active token: %v", err)
	}
	if got.ID != active.ID {
		t.Fatalf("active token id = %d, want %d", got.ID, active.ID)
	}

	if err := st.MarkMagicLoginTokenUsed(ctx, active.ID); err != nil {
		t.Fatalf("mark used: %v", err)
	}
	if _, err := st.FindActiveMagicLoginTokenByEmail(ctx, "admin@example.com", time.Now()); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("used token lookup error = %v, want ErrNotFound", err)
	}
	if err := st.DeleteMagicLoginToken(ctx, expired.ID); err != nil {
		t.Fatalf("delete token: %v", err)
	}
}
