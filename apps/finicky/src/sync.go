package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"finicky/rules"
	"finicky/settingssync"
	"finicky/window"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Cloud operations take only syncMu. In particular they never hold vmMu or
// learningMu, so an unavailable cloud drive cannot stall opening a link.
var syncMu sync.Mutex
var settingsApplyMu sync.Mutex
var activeSettingsSync *settingsSyncCoordinator
var settingsSyncInitError error

type settingsSyncEngine interface {
	LocalStatus() (settingssync.Status, error)
	Status(rules.RulesFile) (settingssync.Status, error)
	Enable(rules.RulesFile) (settingssync.Status, error)
	Disable() error
	SetBindingsFor(rules.RulesFile, map[string]settingssync.Binding) error
	Step(rules.RulesFile) (*settingssync.Update, error)
	Publish(rules.RulesFile, bool) error
	ResolveRemote(rules.RulesFile, string) (*settingssync.Update, error)
	Acknowledge(string) error
}

type settingsSyncCoordinator struct {
	engine       settingsSyncEngine
	cloudPath    string
	blocked      func() string
	load         func() (rules.RulesFile, error)
	apply        func(rules.RulesFile, rules.RulesFile) error
	send         func(string, interface{})
	lastSyncedAt string
}

type settingsSyncSnapshot struct {
	settingssync.Status
	LocalProfiles []rules.SavedProfile `json:"localProfiles"`
	CloudPath     string               `json:"cloudPath,omitempty"`
	BlockedReason string               `json:"blockedReason,omitempty"`
	LastSyncedAt  string               `json:"lastSyncedAt,omitempty"`
	Limitations   []string             `json:"limitations"`
}

// Each callback reads the latest persisted state while holding the application
// lock. A save can finish before an older callback starts; applying that older
// callback's argument would otherwise roll the runtime back to stale settings.
func applyLatestSavedRules(apply func(rules.RulesFile) error) error {
	settingsApplyMu.Lock()
	defer settingsApplyMu.Unlock()
	current, err := rules.Load()
	if err != nil {
		return err
	}
	return apply(current)
}

func settingsSyncBlockedReason() string {
	vmMu.Lock()
	usesJS := vm != nil && vm.IsJSConfig()
	vmMu.Unlock()
	if usesJS {
		return "Sync is paused while a JavaScript configuration controls routing. Move its rules into visual settings first."
	}
	path, err := focusDefaultPath()
	if err != nil {
		return "Sync is paused because this Mac’s Focus settings could not be checked."
	}
	if _, err := os.Lstat(path); err == nil {
		return "Sync is paused while a Focus filter temporarily changes this Mac’s default profile."
	} else if !errors.Is(err, os.ErrNotExist) {
		return "Sync is paused because this Mac’s Focus settings could not be checked."
	}
	return ""
}

func setupSettingsSync() {
	if previewMode || dryRun {
		return
	}
	path, err := rules.GetPath()
	if err != nil {
		settingsSyncInitError = err
		return
	}
	home, err := os.UserHomeDir()
	if err != nil {
		settingsSyncInitError = err
		return
	}
	cloudPath := filepath.Join(home, "Library", "Mobile Documents", "com~apple~CloudDocs", "Finicky Plus Sync")
	activeSettingsSync = &settingsSyncCoordinator{
		engine:    settingssync.New(filepath.Join(filepath.Dir(path), "sync"), cloudPath),
		cloudPath: cloudPath, blocked: settingsSyncBlockedReason, load: rules.Load,
		apply: applySynchronizedRules, send: window.SendMessageToWebView,
	}
	service := activeSettingsSync
	go func() {
		service.perform(map[string]interface{}{"type": "syncNow"}, false)
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			service.perform(map[string]interface{}{"type": "syncNow"}, false)
		}
	}()
}

func applySynchronizedRules(expected, next rules.RulesFile) error {
	if err := rules.SaveIfUnchanged(expected, next); err != nil {
		return err
	}
	if window.SaveRulesHandler != nil {
		if err := window.SaveRulesHandler(next); err != nil {
			return &savedRulesApplyError{err}
		}
	}
	// A later UI save may already have won; always publish the actual disk state.
	current, err := rules.Load()
	if err == nil {
		window.SendMessageToWebView("rules", current)
	}
	return err
}

func settingsSyncStatus(status settingssync.Status, current rules.RulesFile, cloudPath, blocked, lastSynced string) settingsSyncSnapshot {
	if status.Heads == nil {
		status.Heads = []string{}
	}
	if status.HeadDetails == nil {
		status.HeadDetails = []settingssync.HeadInfo{}
	}
	if status.Profiles == nil {
		status.Profiles = []settingssync.ProfileInfo{}
	}
	if status.Bindings == nil {
		status.Bindings = map[string]settingssync.Binding{}
	}
	if status.MissingProfileIDs == nil {
		status.MissingProfileIDs = []string{}
	}
	if status.State == "" {
		status.State = "disabled"
	}
	profiles := append([]rules.SavedProfile{}, current.Profiles...)
	if blocked != "" {
		status.Message = blocked
		if status.Enabled {
			status.State = "pending"
		}
	}
	return settingsSyncSnapshot{
		Status: status, LocalProfiles: profiles, CloudPath: cloudPath, BlockedReason: blocked, LastSyncedAt: lastSynced,
		Limitations: []string{
			"Sync shares visual routing rules, profile labels and supported preferences through your iCloud Drive.",
			"Browser profile folders, learning, history, logs, JavaScript and URL transformation scripts stay on this Mac.",
			"Each Mac needs its own profile mappings, browser extensions and default-browser setup.",
			"Up to date means the revisions currently available on this Mac match; iCloud delivery can take longer.",
		},
	}
}

func decodeSyncBindings(value interface{}) (map[string]settingssync.Binding, error) {
	if _, ok := value.(map[string]interface{}); !ok {
		return nil, fmt.Errorf("profile mappings must be an object")
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	bindings := map[string]settingssync.Binding{}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&bindings); err != nil {
		return nil, fmt.Errorf("invalid profile mappings: %w", err)
	}
	return bindings, nil
}

func (s *settingsSyncCoordinator) applyUpdate(current rules.RulesFile, update *settingssync.Update) error {
	if update == nil {
		return nil
	}
	// A Focus or JS change can occur during an asynchronous cloud read.
	if reason := s.blocked(); reason != "" {
		return errors.New(reason)
	}
	if err := s.apply(current, update.Rules); err != nil {
		return err
	}
	// Remote conflict resolution can publish a merge here. Keep this outside
	// application/runtime locks and acknowledge only a successfully saved apply.
	return s.engine.Acknowledge(update.Revision)
}

func (s *settingsSyncCoordinator) step(current rules.RulesFile) error {
	update, err := s.engine.Step(current)
	if err != nil {
		return err
	}
	return s.applyUpdate(current, update)
}

// Enabling sync or saving mappings succeeds when that local preference reaches
// disk. An existing conflict or offline drive remains visible in status and does
// not turn a completed mapping save into a misleading failure.
func (s *settingsSyncCoordinator) stepWhenReady(current rules.RulesFile) error {
	status, err := s.engine.Status(current)
	if err != nil {
		return err
	}
	if status.State == "synced" || status.State == "localChanges" ||
		(status.State == "pending" && len(status.Heads) == 1 && len(status.MissingProfileIDs) == 0) {
		return s.step(current)
	}
	return nil
}

func (s *settingsSyncCoordinator) perform(msg map[string]interface{}, announce bool) {
	syncMu.Lock()
	defer syncMu.Unlock()
	kind, _ := msg["type"].(string)
	current, readErr := s.load()
	blocked := s.blocked()
	var operationErr error
	enabled, enabledValid := msg["enabled"].(bool)
	disableRequested := kind == "enableSync" && enabledValid && !enabled
	// Turning off sync is always available, including with invalid routing
	// settings, active JavaScript, Focus, or unavailable cloud storage.
	if disableRequested {
		operationErr = s.engine.Disable()
	} else if readErr != nil {
		operationErr = readErr
	} else if blocked != "" && kind != "getSync" {
		operationErr = errors.New(blocked)
	} else {
		switch kind {
		case "getSync":
		case "enableSync":
			if !enabledValid {
				operationErr = fmt.Errorf("an enabled value is required")
				break
			}
			_, operationErr = s.engine.Enable(current)
			if operationErr == nil {
				operationErr = s.stepWhenReady(current)
			}
		case "syncNow":
			operationErr = s.step(current)
		case "saveSyncBindings":
			var bindings map[string]settingssync.Binding
			bindings, operationErr = decodeSyncBindings(msg["bindings"])
			if operationErr == nil {
				operationErr = s.engine.SetBindingsFor(current, bindings)
			}
			if operationErr == nil {
				operationErr = s.stepWhenReady(current)
			}
		case "resolveSync":
			choice, _ := msg["choice"].(string)
			switch choice {
			case "local":
				operationErr = s.engine.Publish(current, true)
			case "remote":
				revision, _ := msg["revision"].(string)
				if revision == "" {
					operationErr = fmt.Errorf("choose the shared revision to use")
					break
				}
				var update *settingssync.Update
				update, operationErr = s.engine.ResolveRemote(current, revision)
				if operationErr == nil {
					operationErr = s.applyUpdate(current, update)
				}
			default:
				operationErr = fmt.Errorf("choose this Mac’s settings or a shared revision")
			}
		default:
			operationErr = fmt.Errorf("unknown sync operation")
		}
	}
	// Status must describe the latest local favorites, including a concurrent
	// user edit or the just-applied incoming revision.
	if latest, err := s.load(); err == nil {
		current = latest
	} else if operationErr == nil && !disableRequested {
		operationErr = err
	}
	blocked = s.blocked()
	var status settingssync.Status
	var statusErr error
	if blocked != "" || readErr != nil {
		status, statusErr = s.engine.LocalStatus()
	} else {
		status, statusErr = s.engine.Status(current)
	}
	if statusErr != nil {
		status.State, status.Message = "error", statusErr.Error()
		if operationErr == nil {
			operationErr = statusErr
		}
	}
	if operationErr != nil && blocked == "" && status.State != "offline" && status.State != "pending" && status.State != "conflict" && status.State != "awaitingMapping" {
		status.State, status.Message = "error", operationErr.Error()
	}
	if status.State == "synced" && operationErr == nil {
		s.lastSyncedAt = time.Now().UTC().Format(time.RFC3339)
	}
	s.send("sync", settingsSyncStatus(status, current, s.cloudPath, blocked, s.lastSyncedAt))
	if announce {
		result := map[string]interface{}{"success": operationErr == nil, "operation": kind}
		if operationErr != nil {
			result["error"] = operationErr.Error()
		}
		s.send("syncResult", result)
	}
}

func handleSyncMessage(msg map[string]interface{}) bool {
	kind, _ := msg["type"].(string)
	switch kind {
	case "getSync", "enableSync", "syncNow", "saveSyncBindings", "resolveSync":
	default:
		return false
	}
	service := activeSettingsSync
	go func() {
		if service != nil && !previewMode && !dryRun {
			service.perform(msg, true)
			return
		}
		reason := "Settings sync is unavailable in preview and dry-run mode."
		if settingsSyncInitError != nil {
			reason = "Settings sync could not start: " + settingsSyncInitError.Error()
		}
		current, _ := rules.Load()
		window.SendMessageToWebView("sync", settingsSyncStatus(settingssync.Status{}, current, "", reason, ""))
		window.SendMessageToWebView("syncResult", map[string]interface{}{"success": kind == "getSync", "operation": kind, "error": reason})
	}()
	return true
}
