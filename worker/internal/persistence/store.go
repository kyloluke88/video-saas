package persistence

import (
	"encoding/json"
	"errors"
	"sync"
	"time"

	"worker/pkg/database"

	"gorm.io/gorm"
)

type Store struct {
	db *gorm.DB
}

var (
	defaultStoreMu sync.Mutex
	defaultStore   *Store
)

func DefaultStore() (*Store, error) {
	defaultStoreMu.Lock()
	defer defaultStoreMu.Unlock()

	if defaultStore != nil {
		return defaultStore, nil
	}
	if !database.Ready() {
		return nil, wrapFatal(errors.New("worker database is not initialized"))
	}
	defaultStore = &Store{db: database.DB}
	return defaultStore, nil
}

func defaultJSON(value json.RawMessage, fallback []byte) json.RawMessage {
	if len(value) == 0 {
		return json.RawMessage(fallback)
	}
	return value
}

func defaultString(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func coalesceString(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func coalesceIntPtr(value *int, fallback *int) *int {
	if value != nil {
		return value
	}
	return fallback
}

func coalesceTimePtr(value *time.Time, fallback *time.Time) *time.Time {
	if value != nil {
		return value
	}
	return fallback
}
