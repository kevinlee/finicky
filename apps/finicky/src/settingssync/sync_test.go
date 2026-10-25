package settingssync

import (
	"bytes"
	"encoding/json"
	"finicky/rules"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func exampleRules() rules.RulesFile {
	yes, no := true, false
	return rules.RulesFile{DefaultBrowser: "Google Chrome", DefaultProfile: "Work local name", DefaultProfileDirectory: "Profile 1", Profiles: []rules.SavedProfile{{ID: "work", Name: "Work", Browser: "Google Chrome", Profile: "Work local name", ProfileDirectory: "Profile 1", Color: "blue", Shortcut: "w"}}, Rules: []rules.Rule{{Name: "Work sites", Match: []string{"*.example.com"}, Browser: "Google Chrome", ProfileDirectory: "Profile 1"}}, Options: &rules.Options{KeepRunning: &yes, HideIcon: &no, LogRequests: &yes, CheckForUpdates: &no}, Features: &rules.Features{HistoryEnabled: true, LearningEnabled: true, PromptMode: "unmatched", AlternativeBrowser: "Safari", NativeAppServices: []string{}}}
}

func engines(t *testing.T) (*Engine, *Engine, string) {
	t.Helper()
	dir := t.TempDir()
	cloud := filepath.Join(dir, "cloud")
	return New(filepath.Join(dir, "a"), cloud), New(filepath.Join(dir, "b"), cloud), cloud
}
func enabled(t *testing.T, e *Engine, rf rules.RulesFile) Status {
	t.Helper()
	s, err := e.Enable(rf)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func mustBindings(t *testing.T, e *Engine, directory string) {
	t.Helper()
	if err := e.SetBindings(map[string]Binding{"work": {Browser: "Google Chrome", Profile: "Local mapped name", ProfileDirectory: directory}}); err != nil {
		t.Fatal(err)
	}
}
func join(t *testing.T, e *Engine, local rules.RulesFile) rules.RulesFile {
	t.Helper()
	if status := enabled(t, e, local); status.State != "awaitingMapping" {
		t.Fatalf("joining guessed mappings: %+v", status)
	}
	mustBindings(t, e, "Profile 7")
	update, err := e.Step(local)
	if err != nil || update == nil {
		t.Fatal(update, err)
	}
	if err := e.Acknowledge(update.Revision); err != nil {
		t.Fatal(err)
	}
	return update.Rules
}

func TestTwoEnginesPropagateAndKeepLocalDataPrivate(t *testing.T) {
	a, b, cloud := engines(t)
	currentA := exampleRules()
	if s := enabled(t, a, currentA); s.State != "synced" {
		t.Fatalf("%+v", s)
	}
	localB := rules.RulesFile{DefaultBrowser: "Safari", Features: &rules.Features{HistoryEnabled: false, LearningEnabled: false}, Options: &rules.Options{}}
	currentB := join(t, b, localB)
	if currentB.DefaultProfileDirectory != "Profile 7" || currentB.Profiles[0].ProfileDirectory != "Profile 7" {
		t.Fatal("used remote profile directory")
	}
	if currentB.Features.HistoryEnabled || currentB.Features.LearningEnabled || currentB.Options.LogRequests != nil || currentB.Options.CheckForUpdates != nil {
		t.Fatal("changed local collection preferences")
	}
	files, err := os.ReadDir(filepath.Join(cloud, "revisions"))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		data, err := os.ReadFile(filepath.Join(cloud, "revisions", file.Name()))
		if err != nil {
			t.Fatal(err)
		}
		for _, secret := range []string{"Profile 1", "Work local name", "profileDirectory", "learningEnabled", "historyEnabled", "logRequests", "checkForUpdates", "transformScript"} {
			if bytes.Contains(data, []byte(secret)) {
				t.Fatalf("cloud contained local field/value %q", secret)
			}
		}
	}
	currentA.Profiles[0].Name = "Research"
	if update, err := a.Step(currentA); err != nil || update != nil {
		t.Fatal(update, err)
	}
	update, err := b.Step(currentB)
	if err != nil || update == nil {
		t.Fatal(update, err)
	}
	if update.Rules.Profiles[0].Name != "Research" || update.Rules.Profiles[0].ID != "work" || update.Rules.Profiles[0].ProfileDirectory != "Profile 7" {
		t.Fatal("rename did not preserve logical/local identity")
	}
	if err := b.Acknowledge(update.Revision); err != nil {
		t.Fatal(err)
	}
	if status, err := b.Status(update.Rules); err != nil || status.State != "synced" {
		t.Fatal(status, err)
	}
}

func TestMissingMappingDoesNotProposeOrGuess(t *testing.T) {
	a, b, _ := engines(t)
	rf := exampleRules()
	enabled(t, a, rf)
	s := enabled(t, b, rf)
	if s.State != "awaitingMapping" || !reflect.DeepEqual(s.MissingProfileIDs, []string{"work"}) || len(s.Bindings) != 0 {
		t.Fatalf("%+v", s)
	}
	if update, err := b.Step(rf); err != nil || update != nil {
		t.Fatal("applied unmapped profile", update, err)
	}
	if _, err := b.ResolveRemote(rf, s.Heads[0]); err == nil {
		t.Fatal("explicit remote choice bypassed mapping")
	}
	if err := b.Acknowledge(s.Heads[0]); err == nil {
		t.Fatal("acknowledged unproposed revision")
	}
}

func TestDirtyLocalAndRemoteRequireExplicitChoice(t *testing.T) {
	a, b, _ := engines(t)
	rfA := exampleRules()
	enabled(t, a, rfA)
	rfB := join(t, b, rules.RulesFile{DefaultBrowser: "Safari"})
	rfA.Rules[0].Name = "A changed"
	if _, err := a.Step(rfA); err != nil {
		t.Fatal(err)
	}
	rfB.Rules[0].Name = "B changed"
	if update, err := b.Step(rfB); err == nil || update != nil {
		t.Fatal("silently overwrote dirty local", update, err)
	}
	status, _ := b.Status(rfB)
	if status.State != "conflict" {
		t.Fatal(status)
	}
	update, err := b.ResolveRemote(rfB, status.Heads[0])
	if err != nil || update == nil {
		t.Fatal(update, err)
	}
	if update.Rules.Rules[0].Name != "A changed" {
		t.Fatal("wrong selected remote content")
	}
	if err := b.Acknowledge(update.Revision); err != nil {
		t.Fatal(err)
	}
}

func createConcurrentHeads(t *testing.T, a, b *Engine, rfA, rfB rules.RulesFile) (string, string) {
	t.Helper()
	g, err := a.scan()
	if err != nil {
		t.Fatal(err)
	}
	sa, _ := a.load()
	sb, _ := b.load()
	rfA.Rules[0].Name = "A branch"
	rfB.Rules[0].Name = "B branch"
	// Both devices observed the same graph before either immutable write arrived.
	if err := a.publish(&sa, rfA, g, false); err != nil {
		t.Fatal(err)
	}
	if err := b.publish(&sb, rfB, g, false); err != nil {
		t.Fatal(err)
	}
	return sa.Applied, sb.Applied
}

func TestConcurrentHeadsNeverChooseNewestAndRemoteMergeWaitsForAck(t *testing.T) {
	a, b, cloud := engines(t)
	rfA := exampleRules()
	enabled(t, a, rfA)
	rfB := join(t, b, rules.RulesFile{DefaultBrowser: "Safari"})
	headA, headB := createConcurrentHeads(t, a, b, rfA, rfB)
	status, _ := a.Status(rfA)
	if status.State != "conflict" || len(status.HeadDetails) != 2 || len(status.Heads) != 2 {
		t.Fatal(status)
	}
	if update, err := a.Step(rfA); err == nil || update != nil {
		t.Fatal("picked a timestamp winner")
	}
	if err := a.Publish(rfA, false); err == nil {
		t.Fatal("implicit concurrent overwrite")
	}
	if _, err := a.ResolveRemote(rfA, ""); err == nil {
		t.Fatal("empty winner selection accepted")
	}
	update, err := a.ResolveRemote(rfA, headB)
	if err != nil {
		t.Fatal(err)
	}
	if update.Rules.Rules[0].Name != "B branch" || update.Rules.Profiles[0].ProfileDirectory != "Profile 1" {
		t.Fatal("remote selection lost local mapping")
	}
	if _, err := os.Stat(filepath.Join(cloud, "revisions", update.Revision+".json")); !os.IsNotExist(err) {
		t.Fatal("published resolution before local apply")
	}
	if err := a.Acknowledge(update.Revision); err != nil {
		t.Fatal(err)
	}
	g, err := a.scan()
	if err != nil {
		t.Fatal(err)
	}
	if len(g.heads) != 1 || g.heads[0] != update.Revision || len(g.revisions[update.Revision].Parents) != 2 {
		t.Fatal("did not join both revision heads")
	}
	if _, ok := g.revisions[headA]; !ok {
		t.Fatal("deleted losing revision")
	}
	if _, ok := g.revisions[headB]; !ok {
		t.Fatal("deleted winning parent")
	}
}

func TestRemoteResolutionDetectsNewChangesBeforeAck(t *testing.T) {
	a, b, cloud := engines(t)
	rfA := exampleRules()
	enabled(t, a, rfA)
	rfB := join(t, b, rules.RulesFile{DefaultBrowser: "Safari"})
	_, headB := createConcurrentHeads(t, a, b, rfA, rfB)
	update, err := a.ResolveRemote(rfA, headB)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Publish(rfB, true); err != nil {
		t.Fatal(err)
	}
	if err := a.Acknowledge(update.Revision); err == nil {
		t.Fatal("resolved stale heads")
	}
	if _, err := os.Stat(filepath.Join(cloud, "revisions", update.Revision+".json")); !os.IsNotExist(err) {
		t.Fatal("stale resolution published")
	}
}

func TestExplicitLocalResolutionPreservesAllParents(t *testing.T) {
	a, b, _ := engines(t)
	rfA := exampleRules()
	enabled(t, a, rfA)
	rfB := join(t, b, rules.RulesFile{DefaultBrowser: "Safari"})
	createConcurrentHeads(t, a, b, rfA, rfB)
	rfA.Rules[0].Name = "Explicit local winner"
	if err := a.Publish(rfA, true); err != nil {
		t.Fatal(err)
	}
	g, err := a.scan()
	if err != nil {
		t.Fatal(err)
	}
	if len(g.heads) != 1 || len(g.revisions[g.heads[0]].Parents) != 2 || g.revisions[g.heads[0]].Settings.Rules[0].Name != "Explicit local winner" {
		t.Fatal("bad local resolution")
	}
}

func TestDisabledAndLocalStatusNeverReadOrCreateCloud(t *testing.T) {
	dir := t.TempDir()
	cloud := filepath.Join(dir, "not-created", "cloud")
	e := New(filepath.Join(dir, "local"), cloud)
	if s, err := e.Status(exampleRules()); err != nil || s.State != "disabled" {
		t.Fatal(s, err)
	}
	if update, err := e.Step(exampleRules()); err != nil || update != nil {
		t.Fatal(update, err)
	}
	mustBindings(t, e, "Profile 3")
	if _, err := os.Stat(filepath.Dir(cloud)); !os.IsNotExist(err) {
		t.Fatal("disabled engine created cloud parent")
	}
	s := enabled(t, e, exampleRules())
	if s.State != "offline" && s.State != "error" {
		t.Fatal(s)
	}
	if s, err := e.LocalStatus(); err != nil || !s.Enabled || s.State != "pending" {
		t.Fatal(s, err)
	}
	if _, err := os.Stat(filepath.Dir(cloud)); !os.IsNotExist(err) {
		t.Fatal("enable fabricated a missing cloud root")
	}
	if err := e.Disable(); err != nil {
		t.Fatal(err)
	}
}

func TestOfflineAndMissingParentHistoryNeverReset(t *testing.T) {
	a, _, cloud := engines(t)
	rf := exampleRules()
	status := enabled(t, a, rf)
	basePath := filepath.Join(cloud, "revisions", status.AppliedRevision+".json")
	parentData, err := os.ReadFile(basePath)
	if err != nil {
		t.Fatal(err)
	}
	rf.Rules[0].Name = "Child"
	if err := a.Publish(rf, false); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(basePath); err != nil {
		t.Fatal(err)
	}
	if s, _ := a.Status(rf); s.State != "pending" {
		t.Fatal("missing parent treated as new history", s)
	}
	if update, err := a.Step(rf); err == nil || update != nil {
		t.Fatal("applied incomplete graph")
	}
	if err := os.WriteFile(basePath, parentData, 0600); err != nil {
		t.Fatal(err)
	}
	if s, _ := a.Status(rf); s.State != "synced" {
		t.Fatal("parent arrival not recovered", s)
	}
	if err := os.Rename(cloud, cloud+"-offline"); err != nil {
		t.Fatal(err)
	}
	if s, _ := a.Status(rf); s.State != "offline" {
		t.Fatal(s)
	}
	if update, err := a.Step(rf); err == nil || update != nil {
		t.Fatal("offline data replaced routing")
	}
	if _, err := os.Stat(cloud); !os.IsNotExist(err) {
		t.Fatal("offline check recreated cloud folder")
	}
}

func TestPlaceholderAndDeletedAppliedRevisionStayPending(t *testing.T) {
	a, _, cloud := engines(t)
	rf := exampleRules()
	status := enabled(t, a, rf)
	placeholder := filepath.Join(cloud, "revisions", "."+strings.Repeat("f", 32)+".json.icloud")
	if err := os.WriteFile(placeholder, []byte{}, 0600); err != nil {
		t.Fatal(err)
	}
	if s, _ := a.Status(rf); s.State != "pending" {
		t.Fatal(s)
	}
	if err := os.Remove(placeholder); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(cloud, "revisions", status.AppliedRevision+".json")); err != nil {
		t.Fatal(err)
	}
	if s := enabled(t, a, rf); s.State != "pending" {
		t.Fatal("deleted history republished silently", s)
	}
	files, _ := os.ReadDir(filepath.Join(cloud, "revisions"))
	if len(files) != 0 {
		t.Fatal("deleted history resurrected")
	}
}

func TestMalformedRevisionAndUnknownFieldsFailClosed(t *testing.T) {
	for _, kind := range []string{"malformed", "unknown", "mismatched-id", "cycle", "oversized", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			a, _, cloud := engines(t)
			rf := exampleRules()
			status := enabled(t, a, rf)
			path := filepath.Join(cloud, "revisions", status.AppliedRevision+".json")
			data, _ := os.ReadFile(path)
			var r revision
			if err := json.Unmarshal(data, &r); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "malformed":
				data = []byte("{")
			case "unknown":
				data = append(data[:len(data)-1], []byte(",\"learningEnabled\":true}")...)
			case "mismatched-id":
				r.ID = strings.Repeat("b", 32)
				data, _ = json.Marshal(r)
			case "cycle":
				r.Parents = []string{r.ID}
				data, _ = json.Marshal(r)
			case "oversized":
				data = bytes.Repeat([]byte(" "), MaxRevisionBytes+1)
			case "symlink":
				os.Remove(path)
				outside := filepath.Join(t.TempDir(), "target")
				os.WriteFile(outside, data, 0600)
				if err := os.Symlink(outside, path); err != nil {
					t.Fatal(err)
				}
			}
			if kind != "symlink" {
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if update, err := a.Step(rf); err == nil || update != nil {
				t.Fatal("malformed revision applied", update, err)
			}
			if s, _ := a.Status(rf); s.State == "synced" || s.State == "pending" {
				t.Fatal("malformed history accepted", s)
			}
		})
	}
}

func TestUnsupportedOutgoingSettingsNeverUpload(t *testing.T) {
	for _, kind := range []string{"script-global", "script-rule", "unsaved-target", "duplicate-id", "path-browser"} {
		t.Run(kind, func(t *testing.T) {
			a, _, cloud := engines(t)
			rf := exampleRules()
			switch kind {
			case "script-global":
				rf.Features.TransformScript = "private_code()"
			case "script-rule":
				rf.Rules[0].TransformScript = "private_code()"
			case "unsaved-target":
				rf.DefaultProfileDirectory = "Profile 99"
			case "duplicate-id":
				rf.Profiles = append(rf.Profiles, rf.Profiles[0])
			case "path-browser":
				rf.Rules[0].Browser = "/private/Custom.app"
				rf.Rules[0].ProfileDirectory = ""
			}
			if s := enabled(t, a, rf); s.State != "error" {
				t.Fatal(s)
			}
			files, _ := os.ReadDir(filepath.Join(cloud, "revisions"))
			if len(files) != 0 {
				t.Fatal("unsupported settings uploaded")
			}
		})
	}
}

func TestExplicitMappingChangeAndValidation(t *testing.T) {
	a, _, _ := engines(t)
	rf := exampleRules()
	enabled(t, a, rf)
	mustBindings(t, a, "Profile 8")
	if status, _ := a.Status(rf); status.State != "pending" {
		t.Fatal("pending explicit mapping shown as synced", status)
	}
	update, err := a.Step(rf)
	if err != nil || update == nil || update.Rules.DefaultProfileDirectory != "Profile 8" {
		t.Fatal(update, err)
	}
	if err := a.Acknowledge(update.Revision); err != nil {
		t.Fatal(err)
	}
	for _, binding := range []Binding{{Browser: "Google Chrome", ProfileDirectory: "../bad"}, {Browser: "Safari", ProfileDirectory: "Default"}, {Browser: "Google Chrome", Profile: "/private/profile"}, {Browser: ""}} {
		if err := a.SetBindings(map[string]Binding{"work": binding}); err == nil {
			t.Fatal("invalid explicit mapping accepted", binding)
		}
	}
	if err := a.SetBindings(map[string]Binding{"work": {Browser: "Google Chrome", ProfileDirectory: "Default"}, "personal": {Browser: "Google Chrome", ProfileDirectory: "Default"}}); err == nil {
		t.Fatal("ambiguous mappings accepted")
	}
}

func retargetLocal(rf *rules.RulesFile, directory string) {
	rf.DefaultProfileDirectory = directory
	rf.Profiles[0].ProfileDirectory = directory
	rf.Rules[0].ProfileDirectory = directory
}

func TestOrdinaryFavoriteEditUpdatesLocalMappingWithoutCloudPublish(t *testing.T) {
	a, _, cloud := engines(t)
	rf := exampleRules()
	status := enabled(t, a, rf)
	retargetLocal(&rf, "Profile 9")
	if update, err := a.Step(rf); err != nil || update != nil {
		t.Fatal("ordinary favorite edit was reverted", update, err)
	}
	local, err := a.LocalStatus()
	if err != nil || local.Bindings["work"].ProfileDirectory != "Profile 9" {
		t.Fatal("new local mapping not retained", local, err)
	}
	files, _ := os.ReadDir(filepath.Join(cloud, "revisions"))
	if len(files) != 1 {
		t.Fatal("machine-local binding uploaded")
	}
	if local.AppliedRevision != status.AppliedRevision {
		t.Fatal("local mapping manufactured shared revision")
	}
	mustBindings(t, a, "Profile 10")
	update, err := a.Step(rf)
	if err != nil || update == nil || update.Rules.DefaultProfileDirectory != "Profile 10" {
		t.Fatal("ordinary refresh erased explicit mapping", update, err)
	}
}

func TestConflictingLocalFavoriteAndMappingEditsAreNotOverwritten(t *testing.T) {
	a, _, _ := engines(t)
	rf := exampleRules()
	enabled(t, a, rf)
	mustBindings(t, a, "Profile 8")
	retargetLocal(&rf, "Profile 9")
	if status, _ := a.Status(rf); status.State != "conflict" || len(status.Profiles) != 1 || len(status.Heads) != 1 || len(status.HeadDetails) != 1 || status.Profiles[0].ID != "work" {
		t.Fatal(status)
	}
	if update, err := a.Step(rf); err == nil || update != nil {
		t.Fatal("silently chose conflicting local mappings", update, err)
	}
	local, _ := a.LocalStatus()
	if local.Bindings["work"].ProfileDirectory != "Profile 8" {
		t.Fatal("explicit pending mapping overwritten")
	}
	mustBindings(t, a, "Profile 9")
	if update, err := a.Step(rf); err != nil || update != nil {
		t.Fatal("explicit matching resolution failed", update, err)
	}
}

func TestMappingConflictRetainsRowsWhenCloudIsOffline(t *testing.T) {
	a, _, cloud := engines(t)
	rf := exampleRules()
	enabled(t, a, rf)
	mustBindings(t, a, "Profile 8")
	retargetLocal(&rf, "Profile 7")
	if err := os.Rename(cloud, cloud+"-offline"); err != nil {
		t.Fatal(err)
	}
	status, err := a.Status(rf)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != "conflict" || status.Message != "local favorite and sync mapping both changed; choose the intended local mapping explicitly" || len(status.Profiles) != 1 || status.Profiles[0].ID != "work" || status.Bindings["work"].ProfileDirectory != "Profile 8" {
		t.Fatal("local mapping cannot be repaired offline", status)
	}
}

func TestSetBindingsForExplicitChoiceOverridesUnpolledEdits(t *testing.T) {
	for _, choice := range []string{"Profile 1", "Profile 9"} {
		t.Run(choice, func(t *testing.T) {
			a, _, _ := engines(t)
			rf := exampleRules()
			enabled(t, a, rf)
			retargetLocal(&rf, "Profile 7")
			// Status may or may not have run; it must not be the observation cutoff.
			before, _ := os.ReadFile(filepath.Join(a.localDir, "state.json"))
			a.Status(rf)
			after, _ := os.ReadFile(filepath.Join(a.localDir, "state.json"))
			if !bytes.Equal(before, after) {
				t.Fatal("Status mutated local baseline")
			}
			bindings := map[string]Binding{"work": {Browser: "Google Chrome", ProfileDirectory: choice}}
			if err := a.SetBindingsFor(rf, bindings); err != nil {
				t.Fatal(err)
			}
			update, err := a.Step(rf)
			if err != nil || update == nil || update.Rules.DefaultProfileDirectory != choice {
				t.Fatal("latest explicit choice lost", update, err)
			}
			if err := a.Acknowledge(update.Revision); err != nil {
				t.Fatal(err)
			}
			if later, err := a.Step(update.Rules); err != nil || later != nil {
				t.Fatal("applied explicit mapping did not settle", later, err)
			}
		})
	}
}

func TestExplicitBindingPreservesConcurrentLocalPortableEdits(t *testing.T) {
	a, _, cloud := engines(t)
	rf := exampleRules()
	enabled(t, a, rf)
	retargetLocal(&rf, "Profile 7")
	rf.Rules[0].Name = "Preserve this local portable edit"
	if err := a.SetBindingsFor(rf, map[string]Binding{"work": {Browser: "Google Chrome", ProfileDirectory: "Profile 9"}}); err != nil {
		t.Fatal(err)
	}
	update, err := a.Step(rf)
	if err != nil || update == nil {
		t.Fatal(update, err)
	}
	if update.Rules.Rules[0].Name != "Preserve this local portable edit" || update.Rules.DefaultProfileDirectory != "Profile 9" || update.Rules.Rules[0].ProfileDirectory != "Profile 9" {
		t.Fatal("binding choice discarded local portable edit", update.Rules)
	}
	if err := a.Acknowledge(update.Revision); err != nil {
		t.Fatal(err)
	}
	if status, _ := a.Status(update.Rules); status.State != "synced" {
		t.Fatal(status)
	}
	files, _ := os.ReadDir(filepath.Join(cloud, "revisions"))
	if len(files) != 2 {
		t.Fatal("expected one portable edit revision", len(files))
	}
}

func TestExplicitRemoteResolutionPreservesUnpolledLocalMappingEdit(t *testing.T) {
	a, b, _ := engines(t)
	rfA := exampleRules()
	enabled(t, a, rfA)
	rfB := join(t, b, rules.RulesFile{DefaultBrowser: "Safari"})
	rfB.Rules[0].Name = "Selected shared rule"
	if _, err := b.Step(rfB); err != nil {
		t.Fatal(err)
	}
	retargetLocal(&rfA, "Profile 9")
	rfA.Rules[0].Name = "Dirty local rule"
	status, err := a.Status(rfA)
	if err != nil || status.State != "conflict" || status.Bindings["work"].ProfileDirectory != "Profile 9" {
		t.Fatal(status, err)
	}
	update, err := a.ResolveRemote(rfA, status.Heads[0])
	if err != nil || update == nil {
		t.Fatal(update, err)
	}
	if update.Rules.DefaultProfileDirectory != "Profile 9" || update.Rules.Profiles[0].ProfileDirectory != "Profile 9" || update.Rules.Rules[0].ProfileDirectory != "Profile 9" || update.Rules.Rules[0].Name != "Selected shared rule" {
		t.Fatal("shared resolution reverted local mapping", update.Rules)
	}
	if err := a.Acknowledge(update.Revision); err != nil {
		t.Fatal(err)
	}
	local, err := a.LocalStatus()
	if err != nil || local.Bindings["work"].ProfileDirectory != "Profile 9" {
		t.Fatal(local, err)
	}
}

func TestImmutableWritesAndPrivateLocalState(t *testing.T) {
	a, _, cloud := engines(t)
	rf := exampleRules()
	s := enabled(t, a, rf)
	path := filepath.Join(cloud, "revisions", s.AppliedRevision+".json")
	before, _ := os.ReadFile(path)
	if err := cloudWrite(path, []byte("replacement")); err == nil {
		t.Fatal("immutable revision overwritten")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("old revision changed")
	}
	for _, item := range []struct {
		path string
		mode os.FileMode
	}{{a.localDir, 0700}, {filepath.Join(a.localDir, "state.json"), 0600}, {path, 0600}} {
		info, err := os.Stat(item.path)
		if err != nil || info.Mode().Perm() != item.mode {
			t.Fatal(item, info, err)
		}
	}
	if s.DeviceID == "" || s.AppliedRevision == "" || len(s.HeadDetails) != 1 || s.HeadDetails[0].CreatedAt.IsZero() {
		t.Fatal("missing provenance", s)
	}
}

func TestAlreadyAppliedRevisionCannotBeMutatedInPlace(t *testing.T) {
	a, _, cloud := engines(t)
	rf := exampleRules()
	status := enabled(t, a, rf)
	path := filepath.Join(cloud, "revisions", status.AppliedRevision+".json")
	data, _ := os.ReadFile(path)
	var r revision
	if err := json.Unmarshal(data, &r); err != nil {
		t.Fatal(err)
	}
	r.Settings.Rules[0].Name = "Unexpected in-place edit"
	data, _ = json.Marshal(r)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if update, err := a.Step(rf); err == nil || update != nil {
		t.Fatal("mutated immutable revision applied", update, err)
	}
	if status, _ := a.Status(rf); status.State != "error" {
		t.Fatal(status)
	}
}

func TestUnknownParentArrivalOrderAndScanBound(t *testing.T) {
	a, _, cloud := engines(t)
	rf := exampleRules()
	enabled(t, a, rf)
	g, _ := a.scan()
	root := g.revisions[g.heads[0]]
	parent := root
	parent.ID = strings.Repeat("c", 32)
	parent.Parents = []string{root.ID}
	parent.CreatedAt = time.Now().Add(-time.Hour)
	child := root
	child.ID = strings.Repeat("d", 32)
	child.Parents = []string{parent.ID}
	child.CreatedAt = time.Now().Add(-2 * time.Hour)
	data, _ := json.Marshal(child)
	if err := cloudWrite(filepath.Join(cloud, "revisions", child.ID+".json"), data); err != nil {
		t.Fatal(err)
	}
	if s, _ := a.Status(rf); s.State != "pending" {
		t.Fatal(s)
	}
	data, _ = json.Marshal(parent)
	if err := cloudWrite(filepath.Join(cloud, "revisions", parent.ID+".json"), data); err != nil {
		t.Fatal(err)
	}
	g, err := a.scan()
	if err != nil || len(g.heads) != 1 || g.heads[0] != child.ID {
		t.Fatal("timestamp used instead of ancestry", g.heads, err)
	}
	// A direct oversized list is bounded even before revision parsing.
	for i := 0; i < MaxRevisions+16; i++ {
		name := filepath.Join(cloud, "revisions", fmtName(i))
		if err := os.WriteFile(name, []byte{}, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := a.scan(); err == nil {
		t.Fatal("unbounded scan accepted")
	}
}

func TestUnknownPlaceholdersAndDuplicatesAreSurfaced(t *testing.T) {
	for _, name := range []string{".unrelated-document.icloud", strings.Repeat("f", 32) + " 2.json"} {
		t.Run(name, func(t *testing.T) {
			a, _, cloud := engines(t)
			rf := exampleRules()
			enabled(t, a, rf)
			if err := os.WriteFile(filepath.Join(cloud, "revisions", name), []byte{}, 0600); err != nil {
				t.Fatal(err)
			}
			status, err := a.Status(rf)
			if err != nil {
				t.Fatal(err)
			}
			if status.State != "conflict" && status.State != "error" {
				t.Fatal("unexpected cloud item was ignored", status)
			}
			if update, err := a.Step(rf); err == nil || update != nil {
				t.Fatal("cloud conflict silently chosen", update, err)
			}
		})
	}
}

func TestSettingsDirectoryPlaceholderIsNotRecreated(t *testing.T) {
	dir := t.TempDir()
	cloud := filepath.Join(dir, "SharedSettings")
	if err := os.WriteFile(filepath.Join(dir, ".SharedSettings.icloud"), []byte{}, 0600); err != nil {
		t.Fatal(err)
	}
	e := New(filepath.Join(dir, "local"), cloud)
	status := enabled(t, e, exampleRules())
	if status.State != "pending" {
		t.Fatal(status)
	}
	if _, err := os.Stat(cloud); !os.IsNotExist(err) {
		t.Fatal("created empty folder over settings placeholder")
	}
}

func TestInitialEnableRecoversWhenCloudParentBecomesAvailable(t *testing.T) {
	dir := t.TempDir()
	parent := filepath.Join(dir, "temporarily-offline")
	cloud := filepath.Join(parent, "SharedSettings")
	e := New(filepath.Join(dir, "local"), cloud)
	rf := exampleRules()
	status := enabled(t, e, rf)
	if !status.Enabled || status.State != "offline" {
		t.Fatal("initial setup should remain enabled offline", status)
	}
	if _, err := os.Stat(parent); !os.IsNotExist(err) {
		t.Fatal("enable fabricated parent")
	}
	if err := os.Mkdir(parent, 0700); err != nil {
		t.Fatal(err)
	}
	if update, err := e.Step(rf); err != nil || update != nil {
		t.Fatal("authorized initial setup did not resume", update, err)
	}
	status, err := e.Status(rf)
	if err != nil || status.State != "synced" || status.AppliedRevision == "" || status.Bindings["work"].ProfileDirectory != "Profile 1" {
		t.Fatal(status, err)
	}
	files, err := os.ReadDir(filepath.Join(cloud, "revisions"))
	if err != nil || len(files) != 1 {
		t.Fatal("initial setup published wrong count", len(files), err)
	}
	if update, err := e.Step(rf); err != nil || update != nil {
		t.Fatal(update, err)
	}
	after, _ := os.ReadDir(filepath.Join(cloud, "revisions"))
	if len(after) != 1 {
		t.Fatal("recovery repeated initial publication")
	}
}

func TestInitialRecoveryDiscoversExistingHistoryWithoutGuessingMappings(t *testing.T) {
	dir := t.TempDir()
	parent := filepath.Join(dir, "temporarily-offline")
	cloud := filepath.Join(parent, "SharedSettings")
	joining := New(filepath.Join(dir, "joining"), cloud)
	rf := exampleRules()
	enabled(t, joining, rf)
	if err := os.Mkdir(parent, 0700); err != nil {
		t.Fatal(err)
	}
	owner := New(filepath.Join(dir, "owner"), cloud)
	ownerStatus := enabled(t, owner, rf)
	if update, err := joining.Step(rf); err != nil || update != nil {
		t.Fatal("recovery applied unmapped profiles", update, err)
	}
	status, err := joining.Status(rf)
	if err != nil || status.State != "awaitingMapping" || len(status.Bindings) != 0 {
		t.Fatal("recovery inferred a joining mapping", status, err)
	}
	files, _ := os.ReadDir(filepath.Join(cloud, "revisions"))
	if len(files) != 1 {
		t.Fatal("recovery created competing root")
	}
	// Even before its first local apply, a device that has seen history must
	// not treat disappearance of those revisions as a new empty sync store.
	if err := os.Remove(filepath.Join(cloud, "revisions", ownerStatus.AppliedRevision+".json")); err != nil {
		t.Fatal(err)
	}
	if update, err := joining.Step(rf); err == nil || update != nil {
		t.Fatal("observed history deletion reset setup", update, err)
	}
	files, _ = os.ReadDir(filepath.Join(cloud, "revisions"))
	if len(files) != 0 {
		t.Fatal("deleted joining history resurrected")
	}
}

func TestPreviouslyAppliedHistoryDoesNotRecreateDeletedCloudDirectories(t *testing.T) {
	e, _, cloud := engines(t)
	rf := exampleRules()
	enabled(t, e, rf)
	if err := os.Rename(cloud, cloud+"-offline"); err != nil {
		t.Fatal(err)
	}
	if update, err := e.Step(rf); err == nil || update != nil {
		t.Fatal(update, err)
	}
	if _, err := os.Stat(cloud); !os.IsNotExist(err) {
		t.Fatal("applied history recreated by recovery")
	}
	if status := enabled(t, e, rf); status.State != "offline" {
		t.Fatal(status)
	}
	if _, err := os.Stat(cloud); !os.IsNotExist(err) {
		t.Fatal("reenable recreated applied history")
	}
}

func fmtName(i int) string {
	const digits = "0123456789abcdef"
	out := make([]byte, 8)
	for j := 7; j >= 0; j-- {
		out[j] = digits[i&15]
		i >>= 4
	}
	return string(out) + ".extra"
}
