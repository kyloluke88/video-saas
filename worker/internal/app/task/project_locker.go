package task

import (
	"strings"
	"sync"
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

func NewProjectLocker() *LocalProjectLocker {
	return &LocalProjectLocker{
		locks: make(map[string]*projectLock),
	}
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

	unlock := l.lock(projectID)
	defer unlock()
	return fn()
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
