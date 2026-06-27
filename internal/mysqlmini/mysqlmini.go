// Package mysqlmini implements a deliberately small database/sql driver for
// MariaDB/MySQL text-protocol queries. It is built for this single-binary
// project to avoid heavyweight ORM/runtime dependencies. It supports the SQL
// operations used by the application: simple COM_QUERY statements, ? argument
// interpolation, transactions, mysql_native_password authentication, OK packets,
// text result sets, DATETIME/DATE parsing, and basic numeric/string types.
package mysqlmini

import (
	"bytes"
	"context"
	"crypto/sha1"
	"database/sql"
	"database/sql/driver"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

func init() { sql.Register("mysqlmini", &Driver{}) }

type Driver struct{}

func (d *Driver) Open(name string) (driver.Conn, error) {
	cfg, err := parseDSN(name)
	if err != nil {
		return nil, err
	}
	c := &Conn{cfg: cfg}
	if err := c.connect(context.Background()); err != nil {
		return nil, err
	}
	return c, nil
}

type config struct {
	user, pass, addr, db string
	timeout              time.Duration
}

func parseDSN(dsn string) (config, error) {
	cfg := config{addr: "127.0.0.1:3306", timeout: 10 * time.Second}
	at := strings.Index(dsn, "@")
	if at < 0 {
		return cfg, fmt.Errorf("DSN should look like user:pass@tcp(host:port)/db")
	}
	auth := dsn[:at]
	rest := dsn[at+1:]
	if i := strings.Index(auth, ":"); i >= 0 {
		cfg.user = auth[:i]
		cfg.pass = auth[i+1:]
	} else {
		cfg.user = auth
	}
	if strings.HasPrefix(rest, "tcp(") {
		end := strings.Index(rest, ")")
		if end < 0 {
			return cfg, fmt.Errorf("bad tcp() in DSN")
		}
		cfg.addr = rest[4:end]
		rest = rest[end+1:]
	}
	if strings.HasPrefix(rest, "/") {
		rest = rest[1:]
	}
	if q := strings.Index(rest, "?"); q >= 0 {
		cfg.db, _ = url.PathUnescape(rest[:q])
		vals, _ := url.ParseQuery(rest[q+1:])
		if t := vals.Get("timeout"); t != "" {
			if d, err := time.ParseDuration(t); err == nil {
				cfg.timeout = d
			}
		}
	} else {
		cfg.db, _ = url.PathUnescape(rest)
	}
	if !strings.Contains(cfg.addr, ":") {
		cfg.addr += ":3306"
	}
	if cfg.user == "" {
		return cfg, fmt.Errorf("DSN user is required")
	}
	if cfg.db == "" {
		return cfg, fmt.Errorf("DSN database is required")
	}
	return cfg, nil
}

type Conn struct {
	cfg    config
	net    net.Conn
	mu     sync.Mutex
	closed bool
}

var _ driver.Conn = (*Conn)(nil)
var _ driver.Pinger = (*Conn)(nil)
var _ driver.ExecerContext = (*Conn)(nil)
var _ driver.QueryerContext = (*Conn)(nil)
var _ driver.ConnBeginTx = (*Conn)(nil)

func (c *Conn) connect(ctx context.Context) error {
	d := net.Dialer{Timeout: c.cfg.timeout}
	nc, err := d.DialContext(ctx, "tcp", c.cfg.addr)
	if err != nil {
		return err
	}
	c.net = nc
	payload, err := c.readPacket()
	if err != nil {
		_ = nc.Close()
		return err
	}
	hs, err := parseHandshake(payload)
	if err != nil {
		_ = nc.Close()
		return err
	}
	if hs.plugin == "" {
		hs.plugin = "mysql_native_password"
	}
	if hs.plugin != "mysql_native_password" {
		_ = nc.Close()
		return fmt.Errorf("server requested auth plugin %q; mysqlmini supports mysql_native_password. For MariaDB set the user plugin accordingly", hs.plugin)
	}
	if err := c.writeHandshakeResponse(hs); err != nil {
		_ = nc.Close()
		return err
	}
	resp, err := c.readPacket()
	if err != nil {
		_ = nc.Close()
		return err
	}
	if len(resp) == 0 {
		_ = nc.Close()
		return io.ErrUnexpectedEOF
	}
	switch resp[0] {
	case 0x00:
		return nil
	case 0xff:
		_ = nc.Close()
		return parseERR(resp)
	case 0xfe: // Auth switch request
		plugin, seed := parseAuthSwitch(resp)
		if plugin != "mysql_native_password" {
			_ = nc.Close()
			return fmt.Errorf("auth switch plugin %q unsupported", plugin)
		}
		if err := c.writePacket(3, nativePassword(c.cfg.pass, seed)); err != nil {
			_ = nc.Close()
			return err
		}
		ok, err := c.readPacket()
		if err != nil {
			_ = nc.Close()
			return err
		}
		if len(ok) > 0 && ok[0] == 0x00 {
			return nil
		}
		if len(ok) > 0 && ok[0] == 0xff {
			_ = nc.Close()
			return parseERR(ok)
		}
		_ = nc.Close()
		return fmt.Errorf("unexpected auth response")
	default:
		_ = nc.Close()
		return fmt.Errorf("unexpected auth packet 0x%x", resp[0])
	}
}

func (c *Conn) Prepare(query string) (driver.Stmt, error) { return &Stmt{c: c, query: query}, nil }
func (c *Conn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	if c.net != nil {
		return c.net.Close()
	}
	return nil
}
func (c *Conn) Begin() (driver.Tx, error)      { return c.BeginTx(context.Background(), driver.TxOptions{}) }
func (c *Conn) Ping(ctx context.Context) error { _, err := c.exec(ctx, "SELECT 1", nil); return err }
func (c *Conn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	_, err := c.exec(ctx, "START TRANSACTION", nil)
	if err != nil {
		return nil, err
	}
	return &Tx{c: c}, nil
}
func (c *Conn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	return c.exec(ctx, query, args)
}
func (c *Conn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	return c.query(ctx, query, args)
}

type Stmt struct {
	c     *Conn
	query string
}

func (s *Stmt) Close() error  { return nil }
func (s *Stmt) NumInput() int { return strings.Count(s.query, "?") }
func (s *Stmt) Exec(args []driver.Value) (driver.Result, error) {
	return s.c.exec(context.Background(), s.query, valuesToNamed(args))
}
func (s *Stmt) Query(args []driver.Value) (driver.Rows, error) {
	return s.c.query(context.Background(), s.query, valuesToNamed(args))
}

type Tx struct{ c *Conn }

func (t *Tx) Commit() error   { _, err := t.c.exec(context.Background(), "COMMIT", nil); return err }
func (t *Tx) Rollback() error { _, err := t.c.exec(context.Background(), "ROLLBACK", nil); return err }

type result struct{ lastID, affected int64 }

func (r result) LastInsertId() (int64, error) { return r.lastID, nil }
func (r result) RowsAffected() (int64, error) { return r.affected, nil }

func valuesToNamed(args []driver.Value) []driver.NamedValue {
	out := make([]driver.NamedValue, len(args))
	for i, v := range args {
		out[i] = driver.NamedValue{Ordinal: i + 1, Value: v}
	}
	return out
}

func (c *Conn) exec(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.setDeadline(ctx); err != nil {
		return nil, err
	}
	q, err := interpolate(query, args)
	if err != nil {
		return nil, err
	}
	if err := c.writePacket(0, append([]byte{0x03}, []byte(q)...)); err != nil {
		return nil, err
	}
	packet, err := c.readPacket()
	if err != nil {
		return nil, err
	}
	if len(packet) == 0 {
		return nil, io.ErrUnexpectedEOF
	}
	if packet[0] == 0xff {
		return nil, parseERR(packet)
	}
	if packet[0] == 0x00 {
		return parseOK(packet)
	}
	// Result set for Exec (e.g. DDL on some servers can still produce metadata): drain it.
	if err := c.drainResult(packet); err != nil {
		return nil, err
	}
	return result{}, nil
}

func (c *Conn) query(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.setDeadline(ctx); err != nil {
		return nil, err
	}
	q, err := interpolate(query, args)
	if err != nil {
		return nil, err
	}
	if err := c.writePacket(0, append([]byte{0x03}, []byte(q)...)); err != nil {
		return nil, err
	}
	packet, err := c.readPacket()
	if err != nil {
		return nil, err
	}
	if len(packet) == 0 {
		return nil, io.ErrUnexpectedEOF
	}
	if packet[0] == 0xff {
		return nil, parseERR(packet)
	}
	if packet[0] == 0x00 {
		return &Rows{cols: []string{}, data: nil}, nil
	}
	colCount, _, err := readLenEncInt(packet, 0)
	if err != nil {
		return nil, err
	}
	cols := make([]column, int(colCount))
	for i := 0; i < int(colCount); i++ {
		p, err := c.readPacket()
		if err != nil {
			return nil, err
		}
		col, err := parseColumn(p)
		if err != nil {
			return nil, err
		}
		cols[i] = col
	}
	p, err := c.readPacket()
	if err != nil {
		return nil, err
	}
	if !isEOF(p) {
		return nil, fmt.Errorf("expected EOF after columns")
	}
	data := [][]driver.Value{}
	for {
		p, err := c.readPacket()
		if err != nil {
			return nil, err
		}
		if isEOF(p) {
			break
		}
		if len(p) > 0 && p[0] == 0xff {
			return nil, parseERR(p)
		}
		row, err := parseTextRow(p, cols)
		if err != nil {
			return nil, err
		}
		data = append(data, row)
	}
	names := make([]string, len(cols))
	for i, c := range cols {
		names[i] = c.name
	}
	return &Rows{cols: names, data: data}, nil
}

func (c *Conn) drainResult(first []byte) error {
	if len(first) == 0 || first[0] == 0x00 {
		return nil
	}
	colCount, _, err := readLenEncInt(first, 0)
	if err != nil {
		return err
	}
	for i := 0; i < int(colCount); i++ {
		if _, err := c.readPacket(); err != nil {
			return err
		}
	}
	p, err := c.readPacket()
	if err != nil {
		return err
	}
	if !isEOF(p) {
		return fmt.Errorf("expected EOF while draining")
	}
	for {
		p, err := c.readPacket()
		if err != nil {
			return err
		}
		if isEOF(p) {
			break
		}
	}
	return nil
}

type Rows struct {
	cols []string
	data [][]driver.Value
	idx  int
}

func (r *Rows) Columns() []string { return r.cols }
func (r *Rows) Close() error      { return nil }
func (r *Rows) Next(dest []driver.Value) error {
	if r.idx >= len(r.data) {
		return io.EOF
	}
	copy(dest, r.data[r.idx])
	r.idx++
	return nil
}

func (c *Conn) setDeadline(ctx context.Context) error {
	if c.closed || c.net == nil {
		return driver.ErrBadConn
	}
	deadline := time.Time{}
	if d, ok := ctx.Deadline(); ok {
		deadline = d
	} else {
		deadline = time.Now().Add(60 * time.Second)
	}
	return c.net.SetDeadline(deadline)
}

func (c *Conn) readPacket() ([]byte, error) {
	header := make([]byte, 4)
	if _, err := io.ReadFull(c.net, header); err != nil {
		return nil, err
	}
	length := int(header[0]) | int(header[1])<<8 | int(header[2])<<16
	payload := make([]byte, length)
	if _, err := io.ReadFull(c.net, payload); err != nil {
		return nil, err
	}
	// Merge split packets if needed.
	for length == 0xffffff {
		h := make([]byte, 4)
		if _, err := io.ReadFull(c.net, h); err != nil {
			return nil, err
		}
		l := int(h[0]) | int(h[1])<<8 | int(h[2])<<16
		p := make([]byte, l)
		if _, err := io.ReadFull(c.net, p); err != nil {
			return nil, err
		}
		payload = append(payload, p...)
		length = l
	}
	return payload, nil
}

func (c *Conn) writePacket(seq byte, payload []byte) error {
	for len(payload) >= 0xffffff {
		if err := c.writeOnePacket(seq, payload[:0xffffff]); err != nil {
			return err
		}
		payload = payload[0xffffff:]
		seq++
	}
	return c.writeOnePacket(seq, payload)
}

func (c *Conn) writeOnePacket(seq byte, payload []byte) error {
	header := []byte{byte(len(payload)), byte(len(payload) >> 8), byte(len(payload) >> 16), seq}
	if _, err := c.net.Write(header); err != nil {
		return err
	}
	_, err := c.net.Write(payload)
	return err
}

type handshake struct {
	seed       []byte
	plugin     string
	capability uint32
}

func parseHandshake(p []byte) (handshake, error) {
	var h handshake
	if len(p) < 34 || p[0] != 10 {
		return h, fmt.Errorf("unsupported handshake")
	}
	pos := 1
	for pos < len(p) && p[pos] != 0 {
		pos++
	}
	pos++ // server version
	pos += 4
	if pos+8 > len(p) {
		return h, io.ErrUnexpectedEOF
	}
	seed1 := append([]byte{}, p[pos:pos+8]...)
	pos += 8
	pos++
	if pos+2 > len(p) {
		return h, io.ErrUnexpectedEOF
	}
	lower := binary.LittleEndian.Uint16(p[pos : pos+2])
	pos += 2
	pos++    // charset
	pos += 2 // status
	if pos+2 > len(p) {
		return h, io.ErrUnexpectedEOF
	}
	upper := binary.LittleEndian.Uint16(p[pos : pos+2])
	pos += 2
	h.capability = uint32(lower) | uint32(upper)<<16
	authLen := 0
	if pos < len(p) {
		authLen = int(p[pos])
		pos++
	}
	pos += 10
	part2Len := 13
	if authLen > 8 {
		part2Len = authLen - 8
	}
	if pos+part2Len > len(p) {
		part2Len = len(p) - pos
	}
	seed2 := []byte{}
	if part2Len > 0 {
		seed2 = append([]byte{}, p[pos:pos+part2Len]...)
		pos += part2Len
	}
	h.seed = bytes.TrimRight(append(seed1, seed2...), "\x00")
	if pos < len(p) {
		start := pos
		for pos < len(p) && p[pos] != 0 {
			pos++
		}
		h.plugin = string(p[start:pos])
	}
	return h, nil
}

func (c *Conn) writeHandshakeResponse(h handshake) error {
	const (
		clientLongPassword     = 1
		clientLongFlag         = 4
		clientConnectWithDB    = 8
		clientProtocol41       = 512
		clientTransactions     = 8192
		clientSecureConnection = 32768
		clientMultiResults     = 131072
		clientPluginAuth       = 1 << 19
	)
	flags := uint32(clientLongPassword | clientLongFlag | clientConnectWithDB | clientProtocol41 | clientTransactions | clientSecureConnection | clientMultiResults | clientPluginAuth)
	var b bytes.Buffer
	_ = binary.Write(&b, binary.LittleEndian, flags)
	_ = binary.Write(&b, binary.LittleEndian, uint32(0))
	b.WriteByte(45) // utf8mb4_general_ci
	b.Write(make([]byte, 23))
	b.WriteString(c.cfg.user)
	b.WriteByte(0)
	authResp := nativePassword(c.cfg.pass, h.seed)
	b.WriteByte(byte(len(authResp)))
	b.Write(authResp)
	b.WriteString(c.cfg.db)
	b.WriteByte(0)
	b.WriteString("mysql_native_password")
	b.WriteByte(0)
	return c.writePacket(1, b.Bytes())
}

func nativePassword(password string, seed []byte) []byte {
	if password == "" {
		return nil
	}
	s1 := sha1.Sum([]byte(password))
	s2 := sha1.Sum(s1[:])
	h := sha1.New()
	h.Write(seed)
	h.Write(s2[:])
	s3 := h.Sum(nil)
	out := make([]byte, len(s1))
	for i := range out {
		out[i] = s1[i] ^ s3[i]
	}
	return out
}

func parseAuthSwitch(p []byte) (plugin string, seed []byte) {
	if len(p) <= 1 {
		return "", nil
	}
	rest := p[1:]
	i := bytes.IndexByte(rest, 0)
	if i < 0 {
		return string(rest), nil
	}
	return string(rest[:i]), bytes.TrimRight(rest[i+1:], "\x00")
}

func parseOK(p []byte) (driver.Result, error) {
	pos := 1
	affected, n, err := readLenEncInt(p, pos)
	if err != nil {
		return nil, err
	}
	pos = n
	lastID, _, err := readLenEncInt(p, pos)
	if err != nil {
		return nil, err
	}
	return result{lastID: int64(lastID), affected: int64(affected)}, nil
}

func parseERR(p []byte) error {
	if len(p) < 3 {
		return fmt.Errorf("mysql error packet")
	}
	code := binary.LittleEndian.Uint16(p[1:3])
	msg := string(p[3:])
	if len(msg) > 6 && msg[0] == '#' {
		msg = msg[6:]
	}
	return fmt.Errorf("mysql %d: %s", code, msg)
}

type column struct {
	name string
	typ  byte
}

func parseColumn(p []byte) (column, error) {
	pos := 0
	for i := 0; i < 6; i++ {
		_, n, err := readLenEncString(p, pos)
		if err != nil {
			return column{}, err
		}
		if i == 4 { // name
			// Re-read cheaply below for clarity.
		}
		pos = n
	}
	// Need actual name; parse again keeping fifth field.
	pos = 0
	name := ""
	for i := 0; i < 6; i++ {
		s, n, err := readLenEncString(p, pos)
		if err != nil {
			return column{}, err
		}
		if i == 4 {
			name = string(s)
		}
		pos = n
	}
	if pos >= len(p) {
		return column{}, io.ErrUnexpectedEOF
	}
	l, n, err := readLenEncInt(p, pos)
	if err != nil {
		return column{}, err
	}
	pos = n + int(l)
	// Actually ColumnDefinition41 has 0x0c length byte then fixed fields. Some servers encode it as length encoded int with value 0x0c.
	if pos > len(p) {
		return column{}, io.ErrUnexpectedEOF
	}
	fixedStart := n
	if int(l) == 12 && fixedStart+13 <= len(p) {
		typ := p[fixedStart+2+4]
		return column{name: name, typ: typ}, nil
	}
	// Fallback for common layout: after 6 lenenc strings, one byte 0x0c, 2 charset, 4 length, 1 type.
	pos2 := 0
	for i := 0; i < 6; i++ {
		_, n2, _ := readLenEncString(p, pos2)
		pos2 = n2
	}
	if pos2+1+2+4 < len(p) {
		return column{name: name, typ: p[pos2+1+2+4]}, nil
	}
	return column{name: name, typ: 0xfd}, nil
}

func isEOF(p []byte) bool { return len(p) < 9 && len(p) > 0 && p[0] == 0xfe }

func parseTextRow(p []byte, cols []column) ([]driver.Value, error) {
	row := make([]driver.Value, len(cols))
	pos := 0
	for i, col := range cols {
		if pos >= len(p) {
			return nil, io.ErrUnexpectedEOF
		}
		if p[pos] == 0xfb {
			row[i] = nil
			pos++
			continue
		}
		raw, n, err := readLenEncString(p, pos)
		if err != nil {
			return nil, err
		}
		pos = n
		row[i] = convertValue(raw, col.typ)
	}
	return row, nil
}

func convertValue(raw []byte, typ byte) driver.Value {
	s := string(raw)
	switch typ {
	case 0x01, 0x02, 0x03, 0x08, 0x09, 0x0d: // integer types
		if n, err := strconv.ParseInt(s, 10, 64); err == nil {
			return n
		}
		if u, err := strconv.ParseUint(s, 10, 64); err == nil && u <= math.MaxInt64 {
			return int64(u)
		}
		return s
	case 0x04, 0x05, 0xf6:
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return f
		}
		return s
	case 0x0a: // DATE
		if t, err := time.ParseInLocation("2006-01-02", s, time.Local); err == nil {
			return t
		}
		return s
	case 0x07, 0x0c: // TIMESTAMP, DATETIME
		layouts := []string{"2006-01-02 15:04:05.999999", "2006-01-02 15:04:05", "2006-01-02T15:04:05Z07:00"}
		for _, layout := range layouts {
			if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
				return t
			}
		}
		return s
	default:
		return s
	}
}

func readLenEncInt(p []byte, pos int) (uint64, int, error) {
	if pos >= len(p) {
		return 0, pos, io.ErrUnexpectedEOF
	}
	first := p[pos]
	pos++
	switch first {
	case 0xfc:
		if pos+2 > len(p) {
			return 0, pos, io.ErrUnexpectedEOF
		}
		return uint64(binary.LittleEndian.Uint16(p[pos : pos+2])), pos + 2, nil
	case 0xfd:
		if pos+3 > len(p) {
			return 0, pos, io.ErrUnexpectedEOF
		}
		return uint64(p[pos]) | uint64(p[pos+1])<<8 | uint64(p[pos+2])<<16, pos + 3, nil
	case 0xfe:
		if pos+8 > len(p) {
			return 0, pos, io.ErrUnexpectedEOF
		}
		return binary.LittleEndian.Uint64(p[pos : pos+8]), pos + 8, nil
	default:
		return uint64(first), pos, nil
	}
}

func readLenEncString(p []byte, pos int) ([]byte, int, error) {
	l, n, err := readLenEncInt(p, pos)
	if err != nil {
		return nil, n, err
	}
	end := n + int(l)
	if end > len(p) {
		return nil, n, io.ErrUnexpectedEOF
	}
	return p[n:end], end, nil
}

func interpolate(query string, args []driver.NamedValue) (string, error) {
	if len(args) == 0 {
		return query, nil
	}
	var b strings.Builder
	argIdx := 0
	inSingle, inDouble, escNext := false, false, false
	for i := 0; i < len(query); i++ {
		ch := query[i]
		if escNext {
			b.WriteByte(ch)
			escNext = false
			continue
		}
		if ch == '\\' {
			b.WriteByte(ch)
			escNext = true
			continue
		}
		if ch == '\'' && !inDouble {
			inSingle = !inSingle
			b.WriteByte(ch)
			continue
		}
		if ch == '"' && !inSingle {
			inDouble = !inDouble
			b.WriteByte(ch)
			continue
		}
		if ch == '?' && !inSingle && !inDouble {
			if argIdx >= len(args) {
				return "", errors.New("not enough SQL arguments")
			}
			b.WriteString(formatArg(args[argIdx].Value))
			argIdx++
			continue
		}
		b.WriteByte(ch)
	}
	if argIdx != len(args) {
		return "", errors.New("too many SQL arguments")
	}
	return b.String(), nil
}

func formatArg(v any) string {
	switch x := v.(type) {
	case nil:
		return "NULL"
	case int64:
		return strconv.FormatInt(x, 10)
	case int:
		return strconv.Itoa(x)
	case int32:
		return strconv.FormatInt(int64(x), 10)
	case uint64:
		return strconv.FormatUint(x, 10)
	case uint:
		return strconv.FormatUint(uint64(x), 10)
	case float64:
		return strconv.FormatFloat(x, 'g', -1, 64)
	case bool:
		if x {
			return "1"
		}
		return "0"
	case time.Time:
		if x.IsZero() {
			return "NULL"
		}
		return "'" + x.Format("2006-01-02 15:04:05") + "'"
	case []byte:
		return "'" + escapeSQL(string(x)) + "'"
	case string:
		return "'" + escapeSQL(x) + "'"
	default:
		return "'" + escapeSQL(fmt.Sprint(x)) + "'"
	}
}

func escapeSQL(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case 0:
			b.WriteString("\\0")
		case '\n':
			b.WriteString("\\n")
		case '\r':
			b.WriteString("\\r")
		case '\\':
			b.WriteString("\\\\")
		case '\'':
			b.WriteString("\\'")
		case '"':
			b.WriteString("\\\"")
		case 26:
			b.WriteString("\\Z")
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}
