// Package cache provides a simple in-memory store for image records and scan state.
package cache

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/yamlwrangler/image-roundup/backend/internal/models"
)

// Store is a thread-safe in-memory data store.
type Store struct {
	mu       sync.RWMutex
	records  map[string]*models.ImageRecord
	lastScan *time.Time
	scanning bool
	errors   []string
}

// New creates an empty Store.
func New() *Store {
	return &Store{records: make(map[string]*models.ImageRecord)}
}

// SetRecords replaces all stored records atomically.
func (s *Store) SetRecords(records []*models.ImageRecord) {
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = make(map[string]*models.ImageRecord, len(records))
	for _, r := range records {
		s.records[r.ID] = r
	}
	s.lastScan = &now
}

// ReplaceWhere replaces records matching shouldReplace with the provided
// records, then updates LastScan. It is used for scoped rescans.
func (s *Store) ReplaceWhere(records []*models.ImageRecord, shouldReplace func(*models.ImageRecord) bool) {
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, r := range s.records {
		if shouldReplace(r) {
			delete(s.records, id)
		}
	}
	for _, r := range records {
		s.records[r.ID] = r
	}
	s.lastScan = &now
}

// UpdateRecord updates or inserts a single record.
func (s *Store) UpdateRecord(r *models.ImageRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records[r.ID] = r
}

// GetRecord returns a single record by ID, or nil.
func (s *Store) GetRecord(id string) *models.ImageRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.records[id]
}

// ListRecords returns a snapshot of all records as a slice.
func (s *Store) ListRecords() []*models.ImageRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*models.ImageRecord, 0, len(s.records))
	for _, r := range s.records {
		out = append(out, r)
	}
	return out
}

// SetScanning updates the in-progress flag.
func (s *Store) SetScanning(v bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scanning = v
}

// SetErrors stores scan-level errors.
func (s *Store) SetErrors(errs []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.errors = errs
}

// ScanStatus returns a point-in-time snapshot of scan state.
func (s *Store) ScanStatus() models.ScanStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	errs := make([]string, len(s.errors))
	copy(errs, s.errors)
	return models.ScanStatus{
		Running:    s.scanning,
		LastScan:   s.lastScan,
		ImageCount: len(s.records),
		Errors:     errs,
	}
}

// LastScan returns the time of the most recent scan completion, or nil.
func (s *Store) LastScan() *time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastScan
}

// Save writes all records to path as newline-delimited JSON.
// The write is atomic: data goes to a temp file that is then renamed.
func (s *Store) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating data dir: %w", err)
	}

	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}

	s.mu.RLock()
	enc := json.NewEncoder(f)
	for _, r := range s.records {
		if err := enc.Encode(r); err != nil {
			s.mu.RUnlock()
			_ = f.Close()
			_ = os.Remove(tmp)
			return fmt.Errorf("encoding record: %w", err)
		}
	}
	s.mu.RUnlock()

	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("closing temp file: %w", err)
	}
	return os.Rename(tmp, path)
}

// Load reads records previously written by Save.
// It is not an error if the file does not exist yet.
func (s *Store) Load(path string) error {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("opening data file: %w", err)
	}
	defer f.Close()

	var records []*models.ImageRecord
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var r models.ImageRecord
		if err := json.Unmarshal(line, &r); err != nil {
			return fmt.Errorf("decoding record: %w", err)
		}
		records = append(records, &r)
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading data file: %w", err)
	}

	s.mu.Lock()
	s.records = make(map[string]*models.ImageRecord, len(records))
	for _, r := range records {
		s.records[r.ID] = r
	}
	s.mu.Unlock()
	return nil
}
