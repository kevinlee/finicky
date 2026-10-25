package rules

import (
	"bytes"
	"encoding/json"
	"errors"
	"path/filepath"
	"sync"
)

// ErrChanged means settings changed after the caller read its expected snapshot.
// Callers should reload and resolve the conflict rather than overwrite the edit.
var ErrChanged = errors.New("settings changed before they could be saved")

var pathWriteLocks sync.Map

func pathWriteLock(path string) *sync.Mutex {
	if absolute, err := filepath.Abs(path); err == nil {
		path = absolute
	}
	lock, _ := pathWriteLocks.LoadOrStore(filepath.Clean(path), &sync.Mutex{})
	return lock.(*sync.Mutex)
}

// SaveIfUnchanged replaces settings only when the current decoded configuration
// still matches expected. Formatting and equivalent legacy JSON representations
// do not cause conflicts. The comparison and backup-preserving save share the
// same process-local lock as Save and SaveToPath. Writers outside this process
// are not coordinated by this guard.
func SaveIfUnchanged(expected, next RulesFile) error {
	path, err := GetPath()
	if err != nil {
		return err
	}
	mu := pathWriteLock(path)
	mu.Lock()
	defer mu.Unlock()

	current, err := LoadFromPath(path)
	if err != nil {
		return err
	}
	currentJSON, err := comparisonJSON(current)
	if err != nil {
		return err
	}
	expectedJSON, err := comparisonJSON(expected)
	if err != nil {
		return err
	}
	if !bytes.Equal(currentJSON, expectedJSON) {
		return ErrChanged
	}
	return saveToPath(next, path)
}

func comparisonJSON(rf RulesFile) ([]byte, error) {
	// Load treats absent/null rules as an empty list. Match also treats absent,
	// null and [] equally; preserve nil vs [] where meaningful, notably the
	// native-app service selection in Features.
	if rf.Rules == nil {
		rf.Rules = []Rule{}
	} else {
		rf.Rules = append([]Rule{}, rf.Rules...)
		for i := range rf.Rules {
			if len(rf.Rules[i].Match) == 0 {
				rf.Rules[i].Match = nil
			}
		}
	}
	return json.Marshal(rf)
}
