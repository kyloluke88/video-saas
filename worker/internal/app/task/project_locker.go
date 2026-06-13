package task

import (
	"context"
	"database/sql"
	"errors"
	"hash/fnv"
	"log"
	"strings"
	"sync"
	"time"

	"worker/pkg/database"
)

type ProjectLocker interface {
	WithProject(projectID string, fn func() error) error
}

type NoopProjectLocker struct{}

func (NoopProjectLocker) WithProject(_ string, fn func() error) error {
	if fn == nil {
		return nil
	}
	return fn()
}

type LocalProjectLocker struct {
	mu    sync.Mutex
	locks map[string]*projectLock
}

type projectLock struct {
	mu   sync.Mutex
	refs int
}

type PostgresProjectLocker struct {
	db *sql.DB
}

func NewProjectLocker() ProjectLocker {
	if database.Ready() && database.SQLDB != nil {
		return NewPostgresProjectLocker(database.SQLDB)
	}
	return NewLocalProjectLocker()
}

func NewLocalProjectLocker() *LocalProjectLocker {
	return &LocalProjectLocker{
		locks: make(map[string]*projectLock),
	}
}

func NewPostgresProjectLocker(db *sql.DB) *PostgresProjectLocker {
	return &PostgresProjectLocker{db: db}
}

func (l *LocalProjectLocker) WithProject(projectID string, fn func() error) error {
	if fn == nil {
		return nil
	}
	if l == nil {
		return fn()
	}

	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return fn()
	}

	waitStartedAt := time.Now()
	log.Printf("🔒 project lock wait backend=local project_id=%s", projectID)
	unlock := l.lock(projectID)
	lockAcquiredAt := time.Now()
	log.Printf("🔒 project lock acquired backend=local project_id=%s wait_ms=%d",
		projectID, lockAcquiredAt.Sub(waitStartedAt).Milliseconds())
	defer func() {
		unlock()
		log.Printf("🔓 project lock released backend=local project_id=%s hold_ms=%d",
			projectID, time.Since(lockAcquiredAt).Milliseconds())
	}()
	return fn()
}

func (l *PostgresProjectLocker) WithProject(projectID string, fn func() error) error {
	if fn == nil {
		return nil
	}
	if l == nil || l.db == nil {
		return fn()
	}

	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return fn()
	}

	conn, err := l.db.Conn(context.Background())
	if err != nil {
		return err
	}
	defer conn.Close()

	lockKey := postgresProjectLockKey(projectID)
	waitStartedAt := time.Now()
	log.Printf("🔒 project lock wait backend=postgres project_id=%s", projectID)
	if _, err := conn.ExecContext(context.Background(), "SELECT pg_advisory_lock($1)", lockKey); err != nil {
		return err
	}
	lockAcquiredAt := time.Now()
	log.Printf("🔒 project lock acquired backend=postgres project_id=%s wait_ms=%d",
		projectID, lockAcquiredAt.Sub(waitStartedAt).Milliseconds())

	runErr := fn()
	if _, unlockErr := conn.ExecContext(context.Background(), "SELECT pg_advisory_unlock($1)", lockKey); unlockErr != nil {
		return errors.Join(runErr, unlockErr)
	}
	log.Printf("🔓 project lock released backend=postgres project_id=%s hold_ms=%d",
		projectID, time.Since(lockAcquiredAt).Milliseconds())
	return runErr
}

func (l *LocalProjectLocker) lock(projectID string) func() {
	l.mu.Lock()
	entry := l.locks[projectID]
	if entry == nil {
		entry = &projectLock{}
		l.locks[projectID] = entry
	}
	entry.refs++
	l.mu.Unlock()

	entry.mu.Lock()

	return func() {
		entry.mu.Unlock()

		l.mu.Lock()
		entry.refs--
		if entry.refs == 0 {
			delete(l.locks, projectID)
		}
		l.mu.Unlock()
	}
}

func postgresProjectLockKey(projectID string) int64 {
	hasher := fnv.New64a()
	_, _ = hasher.Write([]byte(strings.TrimSpace(projectID)))
	return int64(hasher.Sum64())
}
