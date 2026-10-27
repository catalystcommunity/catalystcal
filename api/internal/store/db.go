// Package store is the catalystcal SQLite store: database/sql over the pure-Go
// modernc.org/sqlite driver, with goose migrations from coredb.
//
// Every time column holds TEXT in one layout (TimeLayout), written by
// FormatTime and read by ParseTime. Text comparison and ORDER BY then work as
// time comparison. IDs are ULIDs. Domains are lower-cased on write.
package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/catalystcommunity/catalystcal/coredb"
	"github.com/oklog/ulid/v2"
	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite" // registers the "sqlite" driver
)

// TimeLayout is the one text layout for instants in the database.
const TimeLayout = "2006-01-02T15:04:05Z"

// DateLayout is the layout of all-day dates.
const DateLayout = "2006-01-02"

var (
	// ErrNotFound is returned when a row does not exist.
	ErrNotFound = errors.New("store: not found")
	// ErrConflict is returned when a unique constraint refuses a write.
	ErrConflict = errors.New("store: conflict")
)

// FormatTime writes an instant in TimeLayout (UTC, whole seconds).
func FormatTime(t time.Time) string { return t.UTC().Format(TimeLayout) }

// ParseTime reads an instant written by FormatTime. It also accepts any
// RFC3339 text and converts it to UTC.
func ParseTime(s string) (time.Time, error) {
	t, err := time.Parse(TimeLayout, s)
	if err == nil {
		return t, nil
	}
	t, err2 := time.Parse(time.RFC3339, s)
	if err2 != nil {
		return time.Time{}, fmt.Errorf("bad time %q: %w", s, err)
	}
	return t.UTC().Truncate(time.Second), nil
}

var (
	entropyMu sync.Mutex
	entropy   = ulid.Monotonic(rand.Reader, 0)
)

// NewID returns a new ULID.
func NewID() string {
	entropyMu.Lock()
	defer entropyMu.Unlock()
	return ulid.MustNew(ulid.Timestamp(time.Now()), entropy).String()
}

// DSN builds the modernc SQLite DSN with the pragmas catalystcal needs. The
// pragmas go in the DSN, so every pooled connection gets them.
func DSN(path string) string {
	q := url.Values{}
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "busy_timeout(5000)")
	q.Add("_pragma", "synchronous(NORMAL)")
	q.Set("_txlock", "immediate")
	return "file:" + path + "?" + q.Encode()
}

// Open opens the SQLite database at path. It sets one open connection:
// SQLite serializes writers, and more connections turn waits into
// "database is locked" errors.
func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", DSN(path))
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", path, err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite %s: %w", path, err)
	}
	return db, nil
}

var gooseOnce sync.Once

func gooseSetup() error {
	var err error
	gooseOnce.Do(func() {
		goose.SetBaseFS(coredb.Migrations)
		goose.SetLogger(goose.NopLogger())
		err = goose.SetDialect("sqlite3")
	})
	return err
}

// Migrate applies every pending migration.
func Migrate(ctx context.Context, db *sql.DB) error {
	if err := gooseSetup(); err != nil {
		return err
	}
	return goose.UpContext(ctx, db, "migrations")
}

// Pending reports whether a migration is not applied yet.
func Pending(ctx context.Context, db *sql.DB) (bool, error) {
	if err := gooseSetup(); err != nil {
		return false, err
	}
	migrations, err := goose.CollectMigrations("migrations", 0, goose.MaxVersion)
	if err != nil {
		return false, err
	}
	last, err := migrations.Last()
	if err != nil {
		return false, err
	}
	current, err := goose.GetDBVersionContext(ctx, db)
	if err != nil {
		return false, err
	}
	return current < last.Version, nil
}

// Store is the catalystcal data access layer.
type Store struct {
	DB *sql.DB
	// Now is the clock. Tests can replace it.
	Now func() time.Time
}

// New wraps an open database.
func New(db *sql.DB) *Store {
	return &Store{DB: db, Now: func() time.Time { return time.Now().UTC() }}
}

func (s *Store) now() time.Time { return s.Now().UTC().Truncate(time.Second) }

// Ping checks the database for readiness.
func (s *Store) Ping(ctx context.Context) error { return s.DB.PingContext(ctx) }

// queryer is satisfied by *sql.DB and *sql.Tx.
type queryer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// InTx runs fn in a transaction. fn's error rolls it back.
func (s *Store) InTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// nullStr maps "" to NULL.
func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// nullTime maps nil to NULL.
func nullTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return FormatTime(*t)
}

func strOf(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

func timeOf(ns sql.NullString) *time.Time {
	if !ns.Valid || ns.String == "" {
		return nil
	}
	t, err := ParseTime(ns.String)
	if err != nil {
		return nil
	}
	return &t
}

func mustTime(s string) time.Time {
	t, _ := ParseTime(s)
	return t
}

// isUnique reports a unique-constraint failure from the SQLite driver.
func isUnique(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}

// placeholders returns "?, ?, ?" for n values.
func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.TrimSuffix(strings.Repeat("?, ", n), ", ")
}

func anySlice[T ~string](in []T) []any {
	out := make([]any, len(in))
	for i, v := range in {
		out[i] = string(v)
	}
	return out
}
