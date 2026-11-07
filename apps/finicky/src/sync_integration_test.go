package main

import (
	"encoding/json"
	"errors"
	"finicky/config"
	"finicky/rules"
	"finicky/settingssync"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

type syncEngineFixture struct {
	status   settingssync.Status
	calls    []string
	step     func(rules.RulesFile) (*settingssync.Update, error)
	bindings map[string]settingssync.Binding
}

func (e *syncEngineFixture) LocalStatus() (settingssync.Status, error) {
	e.calls = append(e.calls, "localStatus")
	return e.status, nil
}
func (e *syncEngineFixture) Status(rules.RulesFile) (settingssync.Status, error) {
	e.calls = append(e.calls, "status")
	return e.status, nil
}
func (e *syncEngineFixture) Enable(rules.RulesFile) (settingssync.Status, error) {
	e.calls = append(e.calls, "enable")
	e.status.Enabled = true
	return e.status, nil
}
func (e *syncEngineFixture) Disable() error {
	e.calls = append(e.calls, "disable")
	e.status.Enabled = false
	e.status.State = "disabled"
	return nil
}
func (e *syncEngineFixture) SetBindingsFor(_ rules.RulesFile, v map[string]settingssync.Binding) error {
	e.calls = append(e.calls, "bindings")
	e.bindings = v
	return nil
}
func (e *syncEngineFixture) Step(v rules.RulesFile) (*settingssync.Update, error) {
	e.calls = append(e.calls, "step")
	if e.step != nil {
		return e.step(v)
	}
	return nil, nil
}
func (e *syncEngineFixture) Publish(rules.RulesFile, bool) error {
	e.calls = append(e.calls, "publish")
	return nil
}
func (e *syncEngineFixture) ResolveRemote(v rules.RulesFile, revision string) (*settingssync.Update, error) {
	e.calls = append(e.calls, "resolve:"+revision)
	return &settingssync.Update{Revision: revision, Rules: v}, nil
}
func (e *syncEngineFixture) Acknowledge(revision string) error {
	e.calls = append(e.calls, "ack:"+revision)
	return nil
}

type syncMessages struct{ values map[string]interface{} }

func (m *syncMessages) send(kind string, value interface{}) { m.values[kind] = value }
func (m *syncMessages) result() map[string]interface{} {
	return m.values["syncResult"].(map[string]interface{})
}

func syncIntegrationFixture(t *testing.T, engine settingsSyncEngine) (*settingsSyncCoordinator, *syncMessages, rules.RulesFile) {
	t.Helper()
	rules.SetCustomPath(filepath.Join(t.TempDir(), "rules.json"))
	t.Cleanup(func() { rules.SetCustomPath("") })
	initial := rules.RulesFile{
		DefaultBrowser: "com.google.Chrome", DefaultProfileDirectory: "Profile 2",
		Rules:    []rules.Rule{},
		Profiles: []rules.SavedProfile{{ID: "work", Name: "Work", Browser: "com.google.Chrome", ProfileDirectory: "Profile 2"}},
		Features: &rules.Features{LearningEnabled: true, HistoryEnabled: true},
	}
	if err := rules.Save(initial); err != nil {
		t.Fatal(err)
	}
	messages := &syncMessages{values: map[string]interface{}{}}
	s := &settingsSyncCoordinator{engine: engine, cloudPath: "/synthetic/shared-settings", load: rules.Load,
		blocked: func() string { return "" }, apply: rules.SaveIfUnchanged, send: messages.send}
	return s, messages, initial
}

func TestSyncBridgeAppliesBeforeAcknowledgementAndRejectsConcurrentEdit(t *testing.T) {
	for _, concurrentEdit := range []bool{false, true} {
		t.Run(map[bool]string{false: "apply", true: "concurrent edit"}[concurrentEdit], func(t *testing.T) {
			e := &syncEngineFixture{status: settingssync.Status{Enabled: true, State: "pending"}}
			s, messages, initial := syncIntegrationFixture(t, e)
			e.step = func(current rules.RulesFile) (*settingssync.Update, error) {
				if concurrentEdit {
					current.DefaultBrowser, current.DefaultProfileDirectory = "Safari", ""
					if err := rules.Save(current); err != nil {
						return nil, err
					}
				}
				next := initial
				next.DefaultBrowser, next.DefaultProfileDirectory = "Firefox", ""
				return &settingssync.Update{Revision: "incoming", Rules: next}, nil
			}
			s.apply = func(expected, next rules.RulesFile) error {
				e.calls = append(e.calls, "apply")
				return rules.SaveIfUnchanged(expected, next)
			}
			s.perform(map[string]interface{}{"type": "syncNow"}, true)
			current, err := rules.Load()
			if err != nil {
				t.Fatal(err)
			}
			if concurrentEdit {
				if current.DefaultBrowser != "Safari" || messages.result()["success"] != false || strings.Contains(strings.Join(e.calls, ","), "ack:") {
					t.Fatalf("concurrent edit overwritten or acknowledged: %+v %+v %+v", current, messages.result(), e.calls)
				}
			} else {
				if current.DefaultBrowser != "Firefox" || messages.result()["success"] != true || !reflect.DeepEqual(e.calls, []string{"step", "apply", "ack:incoming", "status"}) {
					t.Fatalf("apply ordering: %+v %+v %+v", current, messages.result(), e.calls)
				}
			}
			if messages.result()["operation"] != "syncNow" {
				t.Fatal("operation not echoed")
			}
		})
	}
}

func TestSyncPauseNeverChecksCloudAndDisableStillWorks(t *testing.T) {
	e := &syncEngineFixture{status: settingssync.Status{Enabled: true}}
	s, messages, _ := syncIntegrationFixture(t, e)
	s.blocked = func() string { return "Focus is active" }
	s.perform(map[string]interface{}{"type": "syncNow"}, true)
	if !reflect.DeepEqual(e.calls, []string{"localStatus"}) || messages.result()["success"] != false {
		t.Fatalf("blocked operation reached cloud: %+v", e.calls)
	}
	status := messages.values["sync"].(settingsSyncSnapshot)
	if status.BlockedReason != "Focus is active" || status.State != "pending" {
		t.Fatalf("missing paused state: %+v", status)
	}
	e.calls = nil
	s.load = func() (rules.RulesFile, error) { return rules.RulesFile{}, errors.New("invalid local settings") }
	s.perform(map[string]interface{}{"type": "enableSync", "enabled": false}, true)
	if !reflect.DeepEqual(e.calls, []string{"disable", "localStatus"}) || messages.result()["success"] != true {
		t.Fatalf("disable was blocked: %+v %+v", e.calls, messages.result())
	}
}

func TestSyncPauseAppearingDuringCloudReadPreventsApply(t *testing.T) {
	e := &syncEngineFixture{status: settingssync.Status{Enabled: true}}
	s, messages, initial := syncIntegrationFixture(t, e)
	blocked := ""
	s.blocked = func() string { return blocked }
	e.step = func(rules.RulesFile) (*settingssync.Update, error) {
		blocked = "JavaScript became active"
		return &settingssync.Update{Revision: "incoming", Rules: rules.RulesFile{DefaultBrowser: "Safari"}}, nil
	}
	s.perform(map[string]interface{}{"type": "syncNow"}, true)
	current, _ := rules.Load()
	if current.DefaultBrowser != initial.DefaultBrowser || messages.result()["success"] != false || !reflect.DeepEqual(e.calls, []string{"step", "localStatus"}) {
		t.Fatalf("late pause was ignored: %+v %+v", current, e.calls)
	}
}

func TestSyncMappingSaveCanSucceedWhileConflictStillNeedsResolution(t *testing.T) {
	e := &syncEngineFixture{status: settingssync.Status{Enabled: true, State: "conflict"}}
	s, messages, _ := syncIntegrationFixture(t, e)
	s.perform(map[string]interface{}{"type": "saveSyncBindings", "bindings": map[string]interface{}{"work": map[string]interface{}{"browser": "com.google.Chrome", "profileDirectory": "Profile 7"}}}, true)
	if messages.result()["success"] != true || e.bindings["work"].ProfileDirectory != "Profile 7" || strings.Contains(strings.Join(e.calls, ","), "step") {
		t.Fatalf("saved mapping misreported: %+v %+v", messages.result(), e.calls)
	}
	if messages.values["sync"].(settingsSyncSnapshot).State != "conflict" {
		t.Fatal("unresolved conflict hidden")
	}
	e.calls = nil
	s.perform(map[string]interface{}{"type": "resolveSync", "choice": "remote"}, true)
	if messages.result()["success"] != false || strings.Contains(strings.Join(e.calls, ","), "resolve:") {
		t.Fatal("remote head inferred without selection")
	}
}

func TestSyncSnapshotHasRequiredContainersAndDoesNotIncludeRules(t *testing.T) {
	snapshot := settingsSyncStatus(settingssync.Status{}, rules.RulesFile{}, "/shared", "", "")
	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]interface{}
	if err = json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"heads", "headDetails", "profiles", "localProfiles", "missingProfileIds", "limitations"} {
		if _, ok := decoded[key].([]interface{}); !ok {
			t.Fatalf("%s is not an array: %s", key, data)
		}
	}
	if _, ok := decoded["bindings"].(map[string]interface{}); !ok {
		t.Fatal("bindings is not an object")
	}
	if _, ok := decoded["rules"]; ok {
		t.Fatal("sync status should not alter the main settings draft")
	}
	if _, err := decodeSyncBindings([]interface{}{}); err == nil {
		t.Fatal("array accepted as bindings")
	}
	if _, err := decodeSyncBindings(map[string]interface{}{"work": map[string]interface{}{"browser": "Safari", "unexpected": "value"}}); err == nil {
		t.Fatal("unknown mapping fields accepted")
	}
}

func TestSettingsCallbackAlwaysReadsLatestSavedRules(t *testing.T) {
	_, _, old := syncIntegrationFixture(t, &syncEngineFixture{})
	latest := old
	latest.DefaultBrowser = "Firefox"
	latest.DefaultProfileDirectory = ""
	if err := rules.Save(latest); err != nil {
		t.Fatal(err)
	}
	var applied string
	handler := func(_ rules.RulesFile) error {
		return applyLatestSavedRules(func(current rules.RulesFile) error { applied = current.DefaultBrowser; return nil })
	}
	if err := handler(old); err != nil {
		t.Fatal(err)
	}
	if applied != "Firefox" {
		t.Fatal("delayed callback applied its stale argument")
	}
}

func TestSyncBlockerChecksActiveJSAndFocusWithoutBrowserData(t *testing.T) {
	_, _, _ = syncIntegrationFixture(t, &syncEngineFixture{})
	vmMu.Lock()
	previous := vm
	vm = nil
	vmMu.Unlock()
	t.Cleanup(func() { vmMu.Lock(); vm = previous; vmMu.Unlock() })
	if reason := settingsSyncBlockedReason(); reason != "" {
		t.Fatal(reason)
	}
	path, _ := focusDefaultPath()
	if err := os.WriteFile(path, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if reason := settingsSyncBlockedReason(); !strings.Contains(reason, "Focus") {
		t.Fatal("Focus was not detected")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	jsPath := filepath.Join(t.TempDir(), "config.js")
	if err := os.WriteFile(jsPath, []byte(`var finickyConfig={default:{defaultBrowser:"Safari"}};`), 0600); err != nil {
		t.Fatal(err)
	}
	jsVM, err := config.New(finickyConfigAPIJS, "finickyConfig", jsPath)
	if err != nil {
		t.Fatal(err)
	}
	vmMu.Lock()
	vm = jsVM
	vmMu.Unlock()
	if reason := settingsSyncBlockedReason(); !strings.Contains(reason, "JavaScript") {
		t.Fatal("active JavaScript was not detected")
	}
}

func TestSyncBridgeReturnsBeforeCloudWorkCompletes(t *testing.T) {
	e := &syncEngineFixture{status: settingssync.Status{Enabled: true, State: "synced"}}
	s, _, _ := syncIntegrationFixture(t, e)
	entered, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
	e.step = func(rules.RulesFile) (*settingssync.Update, error) { close(entered); <-release; return nil, nil }
	s.send = func(kind string, _ interface{}) {
		if kind == "syncResult" {
			close(finished)
		}
	}
	oldService, oldPreview, oldDry := activeSettingsSync, previewMode, dryRun
	activeSettingsSync, previewMode, dryRun = s, false, false
	t.Cleanup(func() { activeSettingsSync, previewMode, dryRun = oldService, oldPreview, oldDry })
	if !handleSyncMessage(map[string]interface{}{"type": "syncNow"}) {
		t.Fatal("sync command not handled")
	}
	select {
	case <-entered:
	case <-time.After(time.Second):
		close(release)
		t.Fatal("background operation did not start")
	}
	select {
	case <-finished:
		close(release)
		t.Fatal("blocked cloud operation unexpectedly finished")
	default:
	}
	close(release)
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("background operation did not complete")
	}
}

func TestSyncIncomingRulesUseThisMacMappingsAndPreserveLocalLearning(t *testing.T) {
	base := t.TempDir()
	cloud := filepath.Join(base, "cloud")
	a := settingssync.New(filepath.Join(base, "device-a"), cloud)
	s, messages, local := syncIntegrationFixture(t, a)
	if _, err := a.Enable(local); err != nil {
		t.Fatal(err)
	}
	b := settingssync.New(filepath.Join(base, "device-b"), cloud)
	remote := local
	remote.DefaultProfileDirectory = "Profile 9"
	remote.Profiles = append([]rules.SavedProfile{}, local.Profiles...)
	remote.Profiles[0].ProfileDirectory = "Profile 9"
	if _, err := b.Enable(remote); err != nil {
		t.Fatal(err)
	}
	if err := b.SetBindings(map[string]settingssync.Binding{"work": {Browser: "com.google.Chrome", ProfileDirectory: "Profile 9"}}); err != nil {
		t.Fatal(err)
	}
	proposal, err := b.Step(remote)
	if err != nil || proposal == nil {
		t.Fatalf("joining proposal: %+v %v", proposal, err)
	}
	remote = proposal.Rules
	if err := b.Acknowledge(proposal.Revision); err != nil {
		t.Fatal(err)
	}
	remote.Rules = []rules.Rule{{Name: "Shared rule", Match: []string{"https://example.com/*"}, Browser: "com.google.Chrome", ProfileDirectory: "Profile 9"}}
	if err := b.Publish(remote, false); err != nil {
		t.Fatal(err)
	}
	s.perform(map[string]interface{}{"type": "syncNow"}, true)
	if messages.result()["success"] != true {
		t.Fatalf("sync failed: %+v", messages.result())
	}
	current, err := rules.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(current.Rules) != 1 || current.Rules[0].ProfileDirectory != "Profile 2" || current.Profiles[0].ProfileDirectory != "Profile 2" {
		t.Fatalf("remote profile directory leaked: %+v", current)
	}
	if current.Features == nil || !current.Features.LearningEnabled || !current.Features.HistoryEnabled {
		t.Fatal("device-local privacy preferences changed")
	}
	// Editing a favorite through normal settings must update the local binding,
	// not get undone by the next poll's older sync mapping.
	current.DefaultProfileDirectory = "Profile 4"
	current.Profiles[0].ProfileDirectory = "Profile 4"
	current.Rules[0].ProfileDirectory = "Profile 4"
	if err := rules.Save(current); err != nil {
		t.Fatal(err)
	}
	s.perform(map[string]interface{}{"type": "syncNow"}, true)
	after, err := rules.Load()
	if err != nil {
		t.Fatal(err)
	}
	if messages.result()["success"] != true || after.Profiles[0].ProfileDirectory != "Profile 4" || after.DefaultProfileDirectory != "Profile 4" {
		t.Fatalf("local destination edit was reversed: %+v %+v", after, messages.result())
	}
}

func TestExplicitSyncMappingWinsOverEarlierUnpolledFavoriteEdit(t *testing.T) {
	base := t.TempDir()
	engine := settingssync.New(filepath.Join(base, "device"), filepath.Join(base, "cloud"))
	s, messages, current := syncIntegrationFixture(t, engine)
	if _, err := engine.Enable(current); err != nil {
		t.Fatal(err)
	}
	// The user first edits the favorite in ordinary settings, then deliberately
	// chooses a different mapping before the next background poll.
	current.DefaultProfileDirectory = "Profile 4"
	current.Profiles[0].ProfileDirectory = "Profile 4"
	if err := rules.Save(current); err != nil {
		t.Fatal(err)
	}
	s.perform(map[string]interface{}{
		"type": "saveSyncBindings", "bindings": map[string]interface{}{
			"work": map[string]interface{}{"browser": "com.google.Chrome", "profileDirectory": "Profile 7"},
		},
	}, true)
	if messages.result()["success"] != true {
		t.Fatalf("deliberate mapping was treated as a concurrent conflict: %+v", messages.result())
	}
	after, err := rules.Load()
	if err != nil {
		t.Fatal(err)
	}
	if after.DefaultProfileDirectory != "Profile 7" || after.Profiles[0].ProfileDirectory != "Profile 7" {
		t.Fatalf("earlier favorite edit overrode deliberate mapping: %+v", after)
	}
	s.perform(map[string]interface{}{"type": "syncNow"}, true)
	stable, err := rules.Load()
	if err != nil || messages.result()["success"] != true || stable.Profiles[0].ProfileDirectory != "Profile 7" {
		t.Fatalf("later poll reversed deliberate mapping: %+v %+v %v", stable, messages.result(), err)
	}
}
