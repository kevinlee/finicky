package linkfeatures

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const HistoryLimit = 200

type HistoryEntry struct {
	MatchedRule      *int      `json:"matchedRule,omitempty"`
	ID               string    `json:"id"`
	URL              string    `json:"url"`
	OriginalURL      string    `json:"originalUrl,omitempty"`
	Browser          string    `json:"browser"`
	Profile          string    `json:"profile,omitempty"`
	ProfileDirectory string    `json:"profileDirectory,omitempty"`
	SourceApp        string    `json:"sourceApp,omitempty"`
	Timestamp        time.Time `json:"timestamp"`
	Error            string    `json:"error,omitempty"`
}

// History is strictly local and caller-controlled; the caller must not add
// private launches or entries while history is disabled.
type Store struct {
	path string
	mu   *sync.Mutex
}

var historyLocks sync.Map

func NewStore(path string) *Store {
	if absolute, err := filepath.Abs(path); err == nil {
		path = absolute
	}
	lock, _ := historyLocks.LoadOrStore(path, &sync.Mutex{})
	return &Store{path: path, mu: lock.(*sync.Mutex)}
}

func DefaultHistoryPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "Finicky Plus", "history.json"), nil
}

func (s *Store) List() ([]HistoryEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read()
}

func (s *Store) read() ([]HistoryEntry, error) {
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return []HistoryEntry{}, nil
	}
	if err != nil {
		return nil, err
	}
	var entries []HistoryEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("could not read link history: %w", err)
	}
	if len(entries) > HistoryLimit {
		entries = entries[:HistoryLimit]
	}
	if entries == nil {
		entries = []HistoryEntry{}
	}
	return entries, nil
}

func (s *Store) Add(entry HistoryEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := s.read()
	if err != nil {
		return err
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now().UTC()
	}
	if entry.ID == "" {
		id := make([]byte, 16)
		if _, err := rand.Read(id); err != nil {
			return err
		}
		entry.ID = hex.EncodeToString(id)
	}
	entries = append([]HistoryEntry{entry}, entries...)
	if len(entries) > HistoryLimit {
		entries = entries[:HistoryLimit]
	}
	return s.write(entries)
}

func (s *Store) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.write([]HistoryEntry{})
}

func (s *Store) write(entries []HistoryEntry) error {
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(s.path), ".finicky-history-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err = file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err = file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), s.path)
}
