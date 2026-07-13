// Package store wraps GORM with typed repositories.
//
// SettingStore is the heart of the dynamic-config system: every
// admin-editable value (AI keys, model list, quotas, feature flags)
// flows through it. Reads are served from an in-memory snapshot so
// they're cheap and lock-free; writes update the database first and
// then refresh the snapshot, so a config change takes effect on the
// very next request without a process restart.
package store

import (
	"errors"
	"sync"

	"gorm.io/gorm"

	"predictdestiny/internal/model"
)

// SettingStore provides typed access to the settings table with an
// in-memory cache for fast, lock-free reads.
type SettingStore struct {
	db *gorm.DB

	mu       sync.RWMutex
	snapshot map[string]string // key -> value (last known good)
}

// NewSettingStore seeds the settings table with defaults (if a key is
// missing) and primes the in-memory snapshot.
func NewSettingStore(db *gorm.DB, defaults []model.Setting) (*SettingStore, error) {
	s := &SettingStore{db: db, snapshot: make(map[string]string)}

	// Ensure a row exists for every default; leave existing rows alone
	// so admin changes survive restarts.
	for _, d := range defaults {
		var existing model.Setting
		err := db.Where("key = ?", d.Key).First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := db.Create(&d).Error; err != nil {
				return nil, err
			}
		} else if err != nil {
			return nil, err
		}
	}

	// Prime the cache from whatever is now in the table.
	var rows []model.Setting
	if err := db.Find(&rows).Error; err != nil {
		return nil, err
	}
	s.mu.Lock()
	for _, r := range rows {
		s.snapshot[r.Key] = r.Value
	}
	s.mu.Unlock()

	return s, nil
}

// Get returns the value for key, or ok=false if it is unset.
func (s *SettingStore) Get(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.snapshot[key]
	return v, ok
}

// GetDefault returns the value for key, or fallback if unset.
func (s *SettingStore) GetDefault(key, fallback string) string {
	if v, ok := s.Get(key); ok && v != "" {
		return v
	}
	return fallback
}

// Set updates a single key in the database and refreshes the cache.
// This is what the admin "Save" button calls.
func (s *SettingStore) Set(key, value string) error {
	if err := s.db.Model(&model.Setting{}).
		Where("key = ?", key).
		Update("value", value).Error; err != nil {
		return err
	}
	s.mu.Lock()
	s.snapshot[key] = value
	s.mu.Unlock()
	return nil
}

// SetMany updates multiple keys in one transaction.
func (s *SettingStore) SetMany(kv map[string]string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		for k, v := range kv {
			if err := tx.Model(&model.Setting{}).
				Where("key = ?", k).
				Update("value", v).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// Reload re-reads every setting from the database into the cache.
// Useful after bulk admin edits or a manual DB change.
func (s *SettingStore) Reload() error {
	var rows []model.Setting
	if err := s.db.Find(&rows).Error; err != nil {
		return err
	}
	s.mu.Lock()
	s.snapshot = make(map[string]string, len(rows))
	for _, r := range rows {
		s.snapshot[r.Key] = r.Value
	}
	s.mu.Unlock()
	return nil
}

// All returns every setting row (for the admin UI).
func (s *SettingStore) All() ([]model.Setting, error) {
	var rows []model.Setting
	err := s.db.Order("sort_order ASC, key ASC").Find(&rows).Error
	return rows, err
}
