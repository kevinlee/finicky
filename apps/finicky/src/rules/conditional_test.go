package rules

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func conditionalTestPath(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "rules.json")
	previous := customPath
	SetCustomPath(path)
	t.Cleanup(func() { SetCustomPath(previous) })
	return path
}

func TestSaveIfUnchangedPreservesBackupAndIgnoresJSONFormatting(t *testing.T) {
	path := conditionalTestPath(t)
	// The input uses the legacy array form for one match and omits rules fields
	// which marshal with explicit zero values. Compare the decoded meaning.
	previous := []byte(`{ "rules": [{"browser":"Safari","match":["example.com/*"]}], "defaultBrowser": "Safari" }`)
	if err := os.WriteFile(path, previous, 0600); err != nil {
		t.Fatal(err)
	}
	expected := RulesFile{DefaultBrowser: "Safari", Rules: []Rule{{Browser: "Safari", Match: []string{"example.com/*"}}}}
	next := RulesFile{DefaultBrowser: "Firefox"}
	if err := SaveIfUnchanged(expected, next); err != nil {
		t.Fatal(err)
	}
	got, err := LoadFromPath(path)
	if err != nil || got.DefaultBrowser != "Firefox" {
		t.Fatalf("new settings = %#v, error %v", got, err)
	}
	backup, err := os.ReadFile(path + ".backup")
	if err != nil || string(backup) != string(previous) {
		t.Fatalf("original backup changed: %q, error %v", backup, err)
	}
}

func TestSaveIfUnchangedRejectsStaleSnapshotWithoutTouchingFiles(t *testing.T) {
	path := conditionalTestPath(t)
	expected := RulesFile{DefaultBrowser: "Safari"}
	if err := SaveToPath(expected, path); err != nil {
		t.Fatal(err)
	}
	if err := SaveToPath(RulesFile{DefaultBrowser: "Firefox"}, path); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	backupBefore, _ := os.ReadFile(path + ".backup")
	if err := SaveIfUnchanged(expected, RulesFile{DefaultBrowser: "Google Chrome"}); !errors.Is(err, ErrChanged) {
		t.Fatalf("stale snapshot error = %v", err)
	}
	after, _ := os.ReadFile(path)
	backupAfter, _ := os.ReadFile(path + ".backup")
	if string(before) != string(after) || string(backupBefore) != string(backupAfter) {
		t.Fatal("conflict changed settings or its backup")
	}
}

func TestSaveIfUnchangedValidatesBeforeMutating(t *testing.T) {
	path := conditionalTestPath(t)
	expected := RulesFile{DefaultBrowser: "Safari"}
	if err := SaveToPath(expected, path); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	invalid := RulesFile{DefaultBrowser: "Google Chrome", DefaultProfileDirectory: "../invalid"}
	if err := SaveIfUnchanged(expected, invalid); err == nil || errors.Is(err, ErrChanged) {
		t.Fatalf("invalid next settings error = %v", err)
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("invalid next settings changed current settings")
	}
	if _, err := os.Stat(path + ".backup"); !os.IsNotExist(err) {
		t.Fatalf("invalid save created backup: %v", err)
	}
}

func TestSaveIfUnchangedEmptyAndCorruptStore(t *testing.T) {
	path := conditionalTestPath(t)
	if err := SaveIfUnchanged(RulesFile{}, RulesFile{DefaultBrowser: "Safari"}); err != nil {
		t.Fatalf("empty initial settings: %v", err)
	}
	corrupt := []byte(`{"defaultBrowser":`)
	if err := os.WriteFile(path, corrupt, 0600); err != nil {
		t.Fatal(err)
	}
	if err := SaveIfUnchanged(RulesFile{}, RulesFile{DefaultBrowser: "Firefox"}); err == nil {
		t.Fatal("corrupt current settings were overwritten")
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(corrupt) {
		t.Fatal("corrupt current settings were changed")
	}
}

func TestSaveIfUnchangedPreservesMeaningfulEmptyServiceSelection(t *testing.T) {
	path := conditionalTestPath(t)
	expected := RulesFile{Features: &Features{NativeApps: true}}
	if err := SaveToPath(expected, path); err != nil {
		t.Fatal(err)
	}
	// nil means all supported native apps, while [] means none.
	expected.Features = &Features{NativeApps: true, NativeAppServices: []string{}}
	if err := SaveIfUnchanged(expected, RulesFile{}); !errors.Is(err, ErrChanged) {
		t.Fatalf("meaningful nil/empty difference was lost: %v", err)
	}
}

func TestSaveIfUnchangedConcurrentWritersHaveOneWinner(t *testing.T) {
	path := conditionalTestPath(t)
	expected := RulesFile{DefaultBrowser: "Safari"}
	if err := SaveToPath(expected, path); err != nil {
		t.Fatal(err)
	}
	const writers = 24
	start := make(chan struct{})
	results := make(chan error, writers)
	var group sync.WaitGroup
	for i := 0; i < writers; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			results <- SaveIfUnchanged(expected, RulesFile{DefaultBrowser: "Firefox"})
		}()
	}
	close(start)
	group.Wait()
	close(results)
	winners := 0
	for err := range results {
		if err == nil {
			winners++
		} else if !errors.Is(err, ErrChanged) {
			t.Fatalf("unexpected save error: %v", err)
		}
	}
	if winners != 1 {
		t.Fatalf("successful concurrent replacements = %d, want 1", winners)
	}
	backup, err := LoadFromPath(path + ".backup")
	if err != nil || backup.DefaultBrowser != "Safari" {
		t.Fatalf("backup = %#v, error %v", backup, err)
	}
}

func TestConditionalAndOrdinarySavesAreSerialized(t *testing.T) {
	path := conditionalTestPath(t)
	expected := RulesFile{DefaultBrowser: "Safari"}
	for attempt := 0; attempt < 16; attempt++ {
		if err := SaveToPath(expected, path); err != nil {
			t.Fatal(err)
		}
		start := make(chan struct{})
		conditional := make(chan error, 1)
		ordinary := make(chan error, 1)
		go func() {
			<-start
			conditional <- SaveIfUnchanged(expected, RulesFile{DefaultBrowser: "Firefox"})
		}()
		go func() {
			<-start
			ordinary <- SaveToPath(RulesFile{DefaultBrowser: "Google Chrome"}, path)
		}()
		close(start)
		conditionalErr, ordinaryErr := <-conditional, <-ordinary
		if ordinaryErr != nil {
			t.Fatal(ordinaryErr)
		}
		wantBackup := "Firefox" // Conditional save ran before the ordinary save.
		if errors.Is(conditionalErr, ErrChanged) {
			wantBackup = "Safari" // Ordinary save won; conditional save did nothing.
		} else if conditionalErr != nil {
			t.Fatal(conditionalErr)
		}
		current, currentErr := LoadFromPath(path)
		backup, backupErr := LoadFromPath(path + ".backup")
		if currentErr != nil || backupErr != nil || current.DefaultBrowser != "Google Chrome" || backup.DefaultBrowser != wantBackup {
			t.Fatalf("nonserial outcome: current=%q backup=%q want backup=%q, errors %v %v", current.DefaultBrowser, backup.DefaultBrowser, wantBackup, currentErr, backupErr)
		}
	}
}
