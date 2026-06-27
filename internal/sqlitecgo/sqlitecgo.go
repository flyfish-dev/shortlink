package sqlitecgo

/*
#cgo LDFLAGS: -lsqlite3
#include <sqlite3.h>
#include <stdlib.h>

static inline int bind_text_transient(sqlite3_stmt* stmt, int idx, const char* value, int len) {
    return sqlite3_bind_text(stmt, idx, value, len, SQLITE_TRANSIENT);
}
static inline int bind_blob_transient(sqlite3_stmt* stmt, int idx, const void* value, int len) {
    return sqlite3_bind_blob(stmt, idx, value, len, SQLITE_TRANSIENT);
}
*/
import "C"

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"
)

func init() {
	sql.Register("sqlitecgo", &drv{})
}

type drv struct{}

type conn struct {
	db *C.sqlite3
	mu sync.Mutex
}

type stmt struct {
	c     *conn
	query string
}

type tx struct{ c *conn }

type rows struct {
	cols []string
	data [][]driver.Value
	idx  int
}

func (d *drv) Open(name string) (driver.Conn, error) {
	if strings.TrimSpace(name) == "" {
		return nil, errors.New("empty sqlite database path")
	}
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))
	var db *C.sqlite3
	flags := C.SQLITE_OPEN_READWRITE | C.SQLITE_OPEN_CREATE | C.SQLITE_OPEN_FULLMUTEX
	if rc := C.sqlite3_open_v2(cname, &db, C.int(flags), nil); rc != C.SQLITE_OK {
		err := sqliteErr(db)
		if db != nil {
			C.sqlite3_close(db)
		}
		return nil, err
	}
	c := &conn{db: db}
	C.sqlite3_busy_timeout(db, 5000)
	for _, pragma := range []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA journal_mode = WAL",
		"PRAGMA synchronous = NORMAL",
		"PRAGMA temp_store = MEMORY",
	} {
		if err := c.execRaw(pragma); err != nil {
			_ = c.Close()
			return nil, err
		}
	}
	return c, nil
}

func (c *conn) Prepare(query string) (driver.Stmt, error) { return &stmt{c: c, query: query}, nil }
func (c *conn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.db == nil {
		return nil
	}
	if rc := C.sqlite3_close(c.db); rc != C.SQLITE_OK {
		return sqliteErr(c.db)
	}
	c.db = nil
	return nil
}
func (c *conn) Begin() (driver.Tx, error) { return c.BeginTx(context.Background(), driver.TxOptions{}) }
func (c *conn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	mode := "DEFERRED"
	if opts.ReadOnly {
		mode = "DEFERRED"
	}
	if err := c.execRaw("BEGIN " + mode); err != nil {
		return nil, err
	}
	return &tx{c: c}, nil
}
func (c *conn) Ping(ctx context.Context) error {
	_, err := c.query(ctx, "SELECT 1", nil)
	return err
}
func (c *conn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	return c.exec(ctx, query, args)
}
func (c *conn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	return c.query(ctx, query, args)
}

func (s *stmt) Close() error  { return nil }
func (s *stmt) NumInput() int { return -1 }
func (s *stmt) Exec(args []driver.Value) (driver.Result, error) {
	return s.ExecContext(context.Background(), valuesToNamed(args))
}
func (s *stmt) Query(args []driver.Value) (driver.Rows, error) {
	return s.QueryContext(context.Background(), valuesToNamed(args))
}
func (s *stmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	return s.c.exec(ctx, s.query, args)
}
func (s *stmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	return s.c.query(ctx, s.query, args)
}

func (t *tx) Commit() error   { return t.c.execRaw("COMMIT") }
func (t *tx) Rollback() error { return t.c.execRaw("ROLLBACK") }

func valuesToNamed(args []driver.Value) []driver.NamedValue {
	out := make([]driver.NamedValue, len(args))
	for i, v := range args {
		out[i] = driver.NamedValue{Ordinal: i + 1, Value: v}
	}
	return out
}

func (c *conn) execRaw(sqlText string) error {
	csql := C.CString(sqlText)
	defer C.free(unsafe.Pointer(csql))
	var errmsg *C.char
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.db == nil {
		return driver.ErrBadConn
	}
	if rc := C.sqlite3_exec(c.db, csql, nil, nil, &errmsg); rc != C.SQLITE_OK {
		if errmsg != nil {
			msg := C.GoString(errmsg)
			C.sqlite3_free(unsafe.Pointer(errmsg))
			return errors.New(msg)
		}
		return sqliteErr(c.db)
	}
	return nil
}

func (c *conn) prepareLocked(query string) (*C.sqlite3_stmt, error) {
	cquery := C.CString(query)
	defer C.free(unsafe.Pointer(cquery))
	var st *C.sqlite3_stmt
	if rc := C.sqlite3_prepare_v2(c.db, cquery, -1, &st, nil); rc != C.SQLITE_OK {
		return nil, sqliteErr(c.db)
	}
	return st, nil
}

func (c *conn) exec(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.db == nil {
		return nil, driver.ErrBadConn
	}
	st, err := c.prepareLocked(query)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(st)
	if err := bindArgs(c.db, st, args); err != nil {
		return nil, err
	}
	for {
		rc := C.sqlite3_step(st)
		switch rc {
		case C.SQLITE_DONE:
			return sqliteResult{lastID: int64(C.sqlite3_last_insert_rowid(c.db)), rows: int64(C.sqlite3_changes(c.db))}, nil
		case C.SQLITE_ROW:
			continue
		case C.SQLITE_BUSY, C.SQLITE_LOCKED:
			return nil, sqliteErr(c.db)
		default:
			return nil, sqliteErr(c.db)
		}
	}
}

func (c *conn) query(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.db == nil {
		return nil, driver.ErrBadConn
	}
	st, err := c.prepareLocked(query)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(st)
	if err := bindArgs(c.db, st, args); err != nil {
		return nil, err
	}
	colN := int(C.sqlite3_column_count(st))
	cols := make([]string, colN)
	decls := make([]string, colN)
	for i := 0; i < colN; i++ {
		cols[i] = C.GoString(C.sqlite3_column_name(st, C.int(i)))
		decls[i] = strings.ToUpper(C.GoString(C.sqlite3_column_decltype(st, C.int(i))))
	}
	out := [][]driver.Value{}
	for {
		rc := C.sqlite3_step(st)
		switch rc {
		case C.SQLITE_ROW:
			row := make([]driver.Value, colN)
			for i := 0; i < colN; i++ {
				row[i] = columnValue(st, i, decls[i])
			}
			out = append(out, row)
		case C.SQLITE_DONE:
			return &rows{cols: cols, data: out, idx: -1}, nil
		default:
			return nil, sqliteErr(c.db)
		}
	}
}

func bindArgs(db *C.sqlite3, st *C.sqlite3_stmt, args []driver.NamedValue) error {
	for i, arg := range args {
		idx := C.int(i + 1)
		if arg.Ordinal > 0 {
			idx = C.int(arg.Ordinal)
		}
		switch v := arg.Value.(type) {
		case nil:
			if rc := C.sqlite3_bind_null(st, idx); rc != C.SQLITE_OK {
				return sqliteErr(db)
			}
		case int64:
			if rc := C.sqlite3_bind_int64(st, idx, C.sqlite3_int64(v)); rc != C.SQLITE_OK {
				return sqliteErr(db)
			}
		case int:
			if rc := C.sqlite3_bind_int64(st, idx, C.sqlite3_int64(v)); rc != C.SQLITE_OK {
				return sqliteErr(db)
			}
		case int32:
			if rc := C.sqlite3_bind_int64(st, idx, C.sqlite3_int64(v)); rc != C.SQLITE_OK {
				return sqliteErr(db)
			}
		case uint64:
			if rc := C.sqlite3_bind_int64(st, idx, C.sqlite3_int64(v)); rc != C.SQLITE_OK {
				return sqliteErr(db)
			}
		case float64:
			if rc := C.sqlite3_bind_double(st, idx, C.double(v)); rc != C.SQLITE_OK {
				return sqliteErr(db)
			}
		case bool:
			iv := int64(0)
			if v {
				iv = 1
			}
			if rc := C.sqlite3_bind_int64(st, idx, C.sqlite3_int64(iv)); rc != C.SQLITE_OK {
				return sqliteErr(db)
			}
		case []byte:
			if len(v) == 0 {
				if rc := C.sqlite3_bind_blob(st, idx, nil, 0, nil); rc != C.SQLITE_OK {
					return sqliteErr(db)
				}
				break
			}
			if rc := C.bind_blob_transient(st, idx, unsafe.Pointer(&v[0]), C.int(len(v))); rc != C.SQLITE_OK {
				return sqliteErr(db)
			}
		case string:
			cs := C.CString(v)
			rc := C.bind_text_transient(st, idx, cs, C.int(len(v)))
			C.free(unsafe.Pointer(cs))
			if rc != C.SQLITE_OK {
				return sqliteErr(db)
			}
		case time.Time:
			s := v.Format("2006-01-02 15:04:05")
			cs := C.CString(s)
			rc := C.bind_text_transient(st, idx, cs, C.int(len(s)))
			C.free(unsafe.Pointer(cs))
			if rc != C.SQLITE_OK {
				return sqliteErr(db)
			}
		default:
			s := fmt.Sprint(v)
			cs := C.CString(s)
			rc := C.bind_text_transient(st, idx, cs, C.int(len(s)))
			C.free(unsafe.Pointer(cs))
			if rc != C.SQLITE_OK {
				return sqliteErr(db)
			}
		}
	}
	return nil
}

func columnValue(st *C.sqlite3_stmt, i int, decl string) driver.Value {
	idx := C.int(i)
	switch C.sqlite3_column_type(st, idx) {
	case C.SQLITE_NULL:
		return nil
	case C.SQLITE_INTEGER:
		return int64(C.sqlite3_column_int64(st, idx))
	case C.SQLITE_FLOAT:
		return float64(C.sqlite3_column_double(st, idx))
	case C.SQLITE_BLOB:
		n := int(C.sqlite3_column_bytes(st, idx))
		ptr := C.sqlite3_column_blob(st, idx)
		if n == 0 || ptr == nil {
			return []byte{}
		}
		return C.GoBytes(ptr, C.int(n))
	default:
		n := int(C.sqlite3_column_bytes(st, idx))
		ptr := (*C.char)(unsafe.Pointer(C.sqlite3_column_text(st, idx)))
		if ptr == nil {
			return ""
		}
		s := C.GoStringN(ptr, C.int(n))
		if isDateDecl(decl) {
			if t, ok := parseTime(s); ok {
				return t
			}
		}
		return s
	}
}

func isDateDecl(decl string) bool {
	return strings.Contains(decl, "DATE") || strings.Contains(decl, "TIME")
}

func parseTime(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	layouts := []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, true
		}
	}
	if sec, err := strconv.ParseInt(s, 10, 64); err == nil && sec > 0 {
		return time.Unix(sec, 0), true
	}
	return time.Time{}, false
}

func (r *rows) Columns() []string { return r.cols }
func (r *rows) Close() error      { return nil }
func (r *rows) Next(dest []driver.Value) error {
	r.idx++
	if r.idx >= len(r.data) {
		return io.EOF
	}
	copy(dest, r.data[r.idx])
	return nil
}

type sqliteResult struct{ lastID, rows int64 }

func (r sqliteResult) LastInsertId() (int64, error) { return r.lastID, nil }
func (r sqliteResult) RowsAffected() (int64, error) { return r.rows, nil }

func sqliteErr(db *C.sqlite3) error {
	if db == nil {
		return errors.New("sqlite error")
	}
	return errors.New(C.GoString(C.sqlite3_errmsg(db)))
}
