package settingssync

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"finicky/rules"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"
)

type Engine struct {
	localDir, cloudDir string
	mu                 *sync.Mutex
}

var engineLocks sync.Map

func New(localDir, cloudDir string) *Engine {
	if p, err := filepath.Abs(localDir); err == nil {
		localDir = p
	}
	if p, err := filepath.Abs(cloudDir); err == nil {
		cloudDir = p
	}
	lock, _ := engineLocks.LoadOrStore(localDir, &sync.Mutex{})
	return &Engine{localDir, cloudDir, lock.(*sync.Mutex)}
}

func newID() (string, error) {
	data := make([]byte, 16)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}
func initialState() state { return state{Version: 1, Bindings: map[string]Binding{}} }
func strictJSON(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("trailing JSON data")
	}
	return nil
}

func (e *Engine) load() (state, error) {
	path := filepath.Join(e.localDir, "state.json")
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return initialState(), nil
	}
	if err != nil {
		return state{}, err
	}
	if !info.Mode().IsRegular() || info.Size() > 3*MaxRevisionBytes {
		return state{}, fmt.Errorf("invalid local sync state")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return state{}, err
	}
	var s state
	if err := strictJSON(data, &s); err != nil {
		return s, fmt.Errorf("invalid local sync state: %w", err)
	}
	if s.Version != 1 || (s.DeviceID != "" && !revisionID.MatchString(s.DeviceID)) || (s.Applied != "" && !revisionID.MatchString(s.Applied)) || (s.Applied != "" && s.Base == nil) {
		return s, fmt.Errorf("unsupported local sync state")
	}
	if s.Bindings == nil {
		s.Bindings = map[string]Binding{}
	}
	for id, b := range s.Bindings {
		if !logicalID.MatchString(id) {
			return s, fmt.Errorf("invalid local profile identity")
		}
		if err := validateBinding(b); err != nil {
			return s, err
		}
	}
	for id, b := range s.AppliedBindings {
		if !logicalID.MatchString(id) {
			return s, fmt.Errorf("invalid applied profile identity")
		}
		if err := validateBinding(b); err != nil {
			return s, err
		}
	}
	if s.Base != nil {
		if err := validatePortable(*s.Base); err != nil {
			return s, err
		}
	}
	if s.Pending != nil {
		if err := validateRevision(*s.Pending); err != nil {
			return s, err
		}
	}
	if s.PendingResolution && s.Pending == nil {
		return s, fmt.Errorf("pending resolution has no proposed revision")
	}
	return s, nil
}

func (e *Engine) save(s state) error {
	if err := os.MkdirAll(e.localDir, 0700); err != nil {
		return err
	}
	info, err := os.Lstat(e.localDir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("local sync state directory must not be a symlink")
	}
	if err := os.Chmod(e.localDir, 0700); err != nil {
		return err
	}
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	if len(data) > 3*MaxRevisionBytes {
		return fmt.Errorf("local sync state exceeds limit")
	}
	file, err := os.CreateTemp(e.localDir, ".state-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if err := file.Chmod(0600); err != nil {
		file.Close()
		return err
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(file.Name(), filepath.Join(e.localDir, "state.json")); err != nil {
		return err
	}
	if dir, err := os.Open(e.localDir); err == nil {
		_ = dir.Sync()
		_ = dir.Close()
	}
	return nil
}

func (e *Engine) Enable(current rules.RulesFile) (Status, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	s, err := e.load()
	if err != nil {
		return Status{}, err
	}
	if s.DeviceID == "" {
		s.DeviceID, err = newID()
		if err != nil {
			return Status{}, err
		}
	}
	s.Enabled = true
	if err := e.save(s); err != nil {
		return Status{}, err
	}
	if _, err := e.scanEnabled(&s, current); err != nil {
		return e.failureStatus(s, err), nil
	}
	return e.status(s, current)
}

// Initial setup remains authorized after Enable returns offline or pending.
// Resume it without a toggle, but never recreate a history this device has
// already seen or applied. A discovered history always follows the join flow.
func (e *Engine) scanEnabled(s *state, current rules.RulesFile) (graph, error) {
	initial := s.Enabled && s.Applied == "" && s.Base == nil && s.Pending == nil && !s.HistoryObserved
	if initial {
		if err := cloudMkdir(e.cloudDir); err != nil {
			return graph{}, err
		}
		if err := cloudMkdir(filepath.Join(e.cloudDir, "revisions")); err != nil {
			return graph{}, err
		}
	}
	g, err := e.scan()
	if len(g.revisions) > 0 && !s.HistoryObserved {
		s.HistoryObserved = true
		if saveErr := e.save(*s); saveErr != nil {
			return g, saveErr
		}
	}
	if err != nil {
		return g, err
	}
	if initial && len(g.revisions) == 0 {
		if err := e.publish(s, current, g, false); err != nil {
			return g, err
		}
		return e.scan()
	}
	return g, nil
}

func (e *Engine) Disable() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	s, err := e.load()
	if err != nil {
		return err
	}
	if !s.Enabled {
		return nil
	}
	s.Enabled = false
	s.Pending = nil
	s.PendingResolution = false
	return e.save(s)
}

func (e *Engine) SetBindings(bindings map[string]Binding) error {
	return e.setBindings(nil, bindings)
}

// SetBindingsFor records the caller's current concrete configuration as the
// observation cutoff for this explicit mapping choice. UI actions should use it
// so a new choice can intentionally override even an unpolled favorite edit.
func (e *Engine) SetBindingsFor(current rules.RulesFile, bindings map[string]Binding) error {
	return e.setBindings(&current, bindings)
}

func (e *Engine) setBindings(current *rules.RulesFile, bindings map[string]Binding) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(bindings) > 100 {
		return fmt.Errorf("too many local profile mappings")
	}
	validated := map[string]Binding{}
	for id, b := range bindings {
		if !logicalID.MatchString(id) {
			return fmt.Errorf("invalid logical profile identity")
		}
		if err := validateBinding(b); err != nil {
			return err
		}
		b.Browser, _ = canonicalBrowser(b.Browser)
		validated[id] = b
	}
	for id, b := range validated {
		for otherID, other := range validated {
			if id != otherID && (b.Profile != "" || b.ProfileDirectory != "") && sameBinding(b, other) {
				return fmt.Errorf("shared profiles must map to distinct local profiles")
			}
		}
	}
	s, err := e.load()
	if err != nil {
		return err
	}
	baselineChanged := false
	if current != nil && s.Base != nil {
		if s.AppliedBindings == nil {
			s.AppliedBindings = map[string]Binding{}
		}
		seen := map[string]bool{}
		for _, p := range current.Profiles {
			if !logicalID.MatchString(p.ID) || seen[p.ID] {
				return fmt.Errorf("current favorites require unique profile identities")
			}
			seen[p.ID] = true
			if _, chosen := validated[p.ID]; !chosen {
				continue
			}
			b := profileBinding(p)
			if err := validateBinding(b); err != nil {
				return err
			}
			b.Browser, _ = canonicalBrowser(b.Browser)
			if old, ok := s.AppliedBindings[p.ID]; !ok || !sameBinding(old, b) {
				s.AppliedBindings[p.ID] = b
				baselineChanged = true
			}
		}
	}
	if reflect.DeepEqual(s.Bindings, validated) && !baselineChanged {
		return nil
	}
	s.Bindings = validated
	s.Pending = nil
	s.PendingResolution = false
	return e.save(s)
}

func (e *Engine) Status(current rules.RulesFile) (Status, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	s, err := e.load()
	if err != nil {
		return Status{State: "error", Message: err.Error()}, err
	}
	return e.status(s, current)
}

// LocalStatus reads only this Mac's private state. Callers can use it while
// Focus or a JavaScript configuration pauses every cloud operation.
func (e *Engine) LocalStatus() (Status, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	s, err := e.load()
	if err != nil {
		return Status{State: "error", Message: err.Error()}, err
	}
	status := baseStatus(s)
	if s.Enabled {
		status.State = "pending"
		status.Message = "Cloud status has not been checked"
	}
	if s.Base != nil {
		addProfiles(&status, *s.Base, s.Bindings)
	}
	return status, nil
}

func baseStatus(s state) Status {
	return Status{Enabled: s.Enabled, State: "disabled", Message: "Settings sync is off", DeviceID: s.DeviceID, AppliedRevision: s.Applied, Heads: []string{}, HeadDetails: []HeadInfo{}, Profiles: []ProfileInfo{}, Bindings: s.Bindings, MissingProfileIDs: []string{}}
}
func (e *Engine) failureStatus(s state, err error) Status {
	status := baseStatus(s)
	status.State = "error"
	status.Message = err.Error()
	var cloud *cloudFailure
	if errors.As(err, &cloud) {
		status.State = cloud.state
		if status.State == "missing" {
			status.State = "offline"
		}
	}
	return status
}

func (e *Engine) status(s state, current rules.RulesFile) (Status, error) {
	status := baseStatus(s)
	if !s.Enabled {
		return status, nil
	}
	_, mappingErr := refreshLocalBindings(&s, current)
	status.Bindings = s.Bindings
	graph, err := e.scan()
	if err != nil {
		if mappingErr != nil {
			if s.Base != nil {
				addProfiles(&status, *s.Base, s.Bindings)
			}
			status.State = "conflict"
			status.Message = mappingErr.Error()
			return status, nil
		}
		return e.failureStatus(s, err), nil
	}
	status.Heads = graph.heads
	for _, id := range graph.heads {
		r := graph.revisions[id]
		status.HeadDetails = append(status.HeadDetails, HeadInfo{r.ID, r.DeviceID, r.CreatedAt})
	}
	if mappingErr != nil {
		addHeadProfiles(&status, graph, s.Bindings)
		if len(status.Profiles) == 0 && s.Base != nil {
			addProfiles(&status, *s.Base, s.Bindings)
		}
		status.State = "conflict"
		status.Message = mappingErr.Error()
		return status, nil
	}
	if err := checkApplied(s, graph); err != nil {
		return e.failureStatus(s, err), nil
	}
	if len(graph.heads) != 1 {
		addHeadProfiles(&status, graph, s.Bindings)
		status.State = "conflict"
		status.Message = "Multiple settings revisions need an explicit choice"
		return status, nil
	}
	head := graph.revisions[graph.heads[0]]
	addProfiles(&status, head.Settings, s.Bindings)
	if len(status.MissingProfileIDs) > 0 {
		status.State = "awaitingMapping"
		status.Message = "Map each shared profile to a destination on this Mac"
		return status, nil
	}
	local, err := exportSettings(current)
	if err != nil {
		return e.failureStatus(s, err), nil
	}
	if s.Base == nil {
		status.State = "pending"
		status.Message = "Shared settings are ready to apply after local profile mapping"
		return status, nil
	}
	remoteChanged := head.ID != s.Applied
	localChanged := !equalSettings(local, *s.Base)
	if localChanged && remoteChanged && !equalSettings(local, head.Settings) {
		status.State = "conflict"
		status.Message = "This Mac and another device both changed settings"
		return status, nil
	}
	if remoteChanged {
		status.State = "pending"
		status.Message = "A shared settings revision is ready to apply"
		return status, nil
	}
	if localChanged {
		status.State = "localChanges"
		status.Message = "Local settings changes are ready to publish"
		return status, nil
	}
	proposal, err := materialize(head.Settings, s.Bindings, current)
	if err != nil {
		return e.failureStatus(s, err), nil
	}
	if !sameLocalSettings(proposal, current) {
		status.State = "pending"
		status.Message = "An explicit local profile mapping is ready to apply"
		return status, nil
	}
	status.State = "synced"
	status.Message = "Portable settings match the latest known revision"
	return status, nil
}

func addProfiles(status *Status, p portable, bindings map[string]Binding) {
	status.MissingProfileIDs = missingBindings(p, bindings)
	for _, v := range p.Profiles {
		_, mapped := bindings[v.ID]
		status.Profiles = append(status.Profiles, ProfileInfo{v.ID, v.Name, v.Color, v.Private, v.Hidden, mapped})
	}
}

func addHeadProfiles(status *Status, g graph, bindings map[string]Binding) {
	seen := map[string]bool{}
	for _, id := range g.heads {
		for _, p := range g.revisions[id].Settings.Profiles {
			if seen[p.ID] {
				continue
			}
			seen[p.ID] = true
			b, ok := bindings[p.ID]
			mapped := ok && validateBinding(b) == nil
			status.Profiles = append(status.Profiles, ProfileInfo{p.ID, p.Name, p.Color, p.Private, p.Hidden, mapped})
			if !mapped {
				status.MissingProfileIDs = append(status.MissingProfileIDs, p.ID)
			}
		}
	}
	sort.Strings(status.MissingProfileIDs)
}

func (e *Engine) Step(current rules.RulesFile) (*Update, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	s, err := e.load()
	if err != nil {
		return nil, err
	}
	if !s.Enabled {
		return nil, nil
	}
	if changed, err := refreshLocalBindings(&s, current); err != nil {
		return nil, err
	} else if changed {
		if err := e.save(s); err != nil {
			return nil, err
		}
	}
	graph, err := e.scanEnabled(&s, current)
	if err != nil {
		return nil, err
	}
	if err := checkApplied(s, graph); err != nil {
		return nil, err
	}
	if len(graph.heads) != 1 {
		return nil, fmt.Errorf("settings sync conflict: resolve concurrent revisions explicitly")
	}
	head := graph.revisions[graph.heads[0]]
	if len(missingBindings(head.Settings, s.Bindings)) > 0 {
		return nil, nil
	}
	local, err := exportSettings(current)
	if err != nil {
		return nil, err
	}
	remoteChanged := head.ID != s.Applied
	localChanged := s.Base != nil && !equalSettings(local, *s.Base)
	if remoteChanged && localChanged && !equalSettings(local, head.Settings) {
		return nil, fmt.Errorf("settings sync conflict: both local and shared settings changed")
	}
	if !remoteChanged && localChanged {
		pendingMapping := false
		for _, p := range current.Profiles {
			if mapped, ok := s.Bindings[p.ID]; ok && !sameBinding(mapped, profileBinding(p)) {
				pendingMapping = true
			}
		}
		if err := e.publish(&s, current, graph, false); err != nil {
			return nil, err
		}
		if !pendingMapping {
			return nil, nil
		}
		graph, err = e.scan()
		if err != nil {
			return nil, err
		}
		if len(graph.heads) != 1 || graph.heads[0] != s.Applied {
			return nil, fmt.Errorf("settings changed while publishing; explicit local mapping remains pending")
		}
		head = graph.revisions[s.Applied]
	}
	proposal, err := materialize(head.Settings, s.Bindings, current)
	if err != nil {
		return nil, err
	}
	if !remoteChanged && sameLocalSettings(proposal, current) {
		return nil, nil
	}
	s.Pending = &head
	s.PendingResolution = false
	if err := e.save(s); err != nil {
		return nil, err
	}
	return &Update{Revision: head.ID, Rules: proposal}, nil
}

// Acknowledge must run only after the caller successfully compare-and-saves the
// proposed RulesFile. It does not need cloud access, so a completed local apply
// can be acknowledged if iCloud goes offline immediately afterward.
func (e *Engine) Acknowledge(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	s, err := e.load()
	if err != nil {
		return err
	}
	if !s.Enabled {
		return fmt.Errorf("settings sync is disabled")
	}
	if s.Pending == nil || s.Pending.ID != id {
		return fmt.Errorf("revision was not proposed for local application")
	}
	if s.PendingResolution {
		graph, err := e.scan()
		if err != nil {
			return err
		}
		if existing, ok := graph.revisions[id]; ok {
			expected, _ := json.Marshal(s.Pending)
			actual, _ := json.Marshal(existing)
			if !bytes.Equal(expected, actual) {
				return fmt.Errorf("pending resolution identity already has different content")
			}
		} else {
			if !reflect.DeepEqual(graph.heads, s.Pending.Parents) {
				return fmt.Errorf("settings changed during conflict resolution; choose again")
			}
			if len(graph.revisions) >= MaxRevisions {
				return fmt.Errorf("settings revision limit reached")
			}
			data, err := json.Marshal(s.Pending)
			if err != nil {
				return err
			}
			if len(data) > MaxRevisionBytes {
				return fmt.Errorf("resolution exceeds size limit")
			}
			if err := cloudWrite(filepath.Join(e.cloudDir, "revisions", id+".json"), data); err != nil {
				return err
			}
		}
	}
	s.Applied = id
	s.HistoryObserved = true
	s.Base = &s.Pending.Settings
	s.AppliedBindings = bindingsFor(*s.Base, s.Bindings)
	s.Pending = nil
	s.PendingResolution = false
	return e.save(s)
}

// ResolveRemote is an explicit user choice, not automatic conflict handling.
// When several heads exist, the selected content is prepared as a merge whose
// cloud publication is deferred until Acknowledge follows a successful local
// compare-and-save. Timestamps never choose a winner.
func (e *Engine) ResolveRemote(current rules.RulesFile, id string) (*Update, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	s, err := e.load()
	if err != nil {
		return nil, err
	}
	if !s.Enabled {
		return nil, fmt.Errorf("settings sync is disabled")
	}
	if _, err := refreshLocalBindings(&s, current); err != nil {
		return nil, err
	}
	graph, err := e.scan()
	if err != nil {
		return nil, err
	}
	if err := checkApplied(s, graph); err != nil {
		return nil, err
	}
	selected := false
	for _, head := range graph.heads {
		if head == id {
			selected = true
		}
	}
	if !selected {
		return nil, fmt.Errorf("select one of the current settings revision heads")
	}
	r := graph.revisions[id]
	proposal, err := materialize(r.Settings, s.Bindings, current)
	if err != nil {
		return nil, err
	}
	s.PendingResolution = len(graph.heads) > 1
	if s.PendingResolution {
		if len(graph.revisions) >= MaxRevisions {
			return nil, fmt.Errorf("settings revision limit reached")
		}
		newID, err := newID()
		if err != nil {
			return nil, err
		}
		r = revision{Version: 1, ID: newID, DeviceID: s.DeviceID, Parents: append([]string{}, graph.heads...), CreatedAt: time.Now().UTC(), Settings: r.Settings}
	}
	s.Pending = &r
	if err := e.save(s); err != nil {
		return nil, err
	}
	return &Update{Revision: r.ID, Rules: proposal}, nil
}

func sameLocalSettings(a, b rules.RulesFile) bool {
	// Ignore nil versus empty slices and option containers by comparing a fully
	// materialized portable projection plus concrete profile bindings.
	aPortable, err := exportSettings(a)
	if err != nil {
		return false
	}
	bPortable, err := exportSettings(b)
	if err != nil {
		return false
	}
	if !equalSettings(aPortable, bPortable) {
		return false
	}
	if !sameBinding(Binding{a.DefaultBrowser, a.DefaultProfile, a.DefaultProfileDirectory}, Binding{b.DefaultBrowser, b.DefaultProfile, b.DefaultProfileDirectory}) {
		return false
	}
	for i := range a.Profiles {
		if !sameBinding(profileBinding(a.Profiles[i]), profileBinding(b.Profiles[i])) {
			return false
		}
	}
	for i := range a.Rules {
		if !sameBinding(Binding{a.Rules[i].Browser, a.Rules[i].Profile, a.Rules[i].ProfileDirectory}, Binding{b.Rules[i].Browser, b.Rules[i].Profile, b.Rules[i].ProfileDirectory}) {
			return false
		}
	}
	if a.Features != nil && b.Features != nil {
		return sameBinding(Binding{a.Features.AlternativeBrowser, a.Features.AlternativeProfile, a.Features.AlternativeProfileDirectory}, Binding{b.Features.AlternativeBrowser, b.Features.AlternativeProfile, b.Features.AlternativeProfileDirectory})
	}
	return true
}

func (e *Engine) Publish(current rules.RulesFile, resolve bool) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	s, err := e.load()
	if err != nil {
		return err
	}
	if !s.Enabled {
		return fmt.Errorf("settings sync is disabled")
	}
	graph, err := e.scan()
	if err != nil {
		return err
	}
	if err := checkApplied(s, graph); err != nil {
		return err
	}
	return e.publish(&s, current, graph, resolve)
}

func (e *Engine) publish(s *state, current rules.RulesFile, graph graph, resolve bool) error {
	if len(graph.revisions) >= MaxRevisions {
		return fmt.Errorf("revision limit reached; preserve and reset sync history explicitly before publishing more")
	}
	settings, err := exportSettings(current)
	if err != nil {
		return err
	}
	if !resolve {
		if len(graph.heads) > 1 {
			return fmt.Errorf("concurrent revisions require explicit conflict resolution")
		}
		if len(graph.heads) == 1 && graph.heads[0] != s.Applied {
			return fmt.Errorf("apply incoming shared settings before publishing local changes")
		}
	}
	id, err := newID()
	if err != nil {
		return err
	}
	parents := append([]string{}, graph.heads...)
	sort.Strings(parents)
	r := revision{Version: 1, ID: id, DeviceID: s.DeviceID, Parents: parents, CreatedAt: time.Now().UTC(), Settings: settings}
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}
	if len(data) > MaxRevisionBytes {
		return fmt.Errorf("settings revision exceeds size limit")
	}
	if err := cloudWrite(filepath.Join(e.cloudDir, "revisions", id+".json"), data); err != nil {
		return err
	}
	// The local config was the source of portable content. An explicit local
	// mapping can still be pending; publishing must not erase that choice.
	oldBindings, oldApplied, hadBase := s.Bindings, s.AppliedBindings, s.Base != nil
	s.Applied = id
	s.HistoryObserved = true
	s.Base = &settings
	s.Pending = nil
	s.PendingResolution = false
	s.Bindings = map[string]Binding{}
	actualBindings := map[string]Binding{}
	for _, p := range current.Profiles {
		b := profileBinding(p)
		b.Browser, _ = canonicalBrowser(b.Browser)
		s.Bindings[p.ID] = b
		actualBindings[p.ID] = b
		if !resolve && hadBase {
			if desired, ok := oldBindings[p.ID]; ok {
				if previous, known := oldApplied[p.ID]; known && !sameBinding(desired, previous) {
					s.Bindings[p.ID] = desired
				}
			}
		}
	}
	s.AppliedBindings = actualBindings
	return e.save(*s)
}

func bindingsFor(p portable, bindings map[string]Binding) map[string]Binding {
	result := map[string]Binding{}
	for _, profile := range p.Profiles {
		if b, ok := bindings[profile.ID]; ok {
			result[profile.ID] = b
		}
	}
	return result
}

// A local favorite edit and an explicit sync mapping edit are distinct inputs.
// Compare both against the last actually applied concrete bindings so neither
// silently undoes the other. No baseline means no inference on a joining Mac.
func refreshLocalBindings(s *state, current rules.RulesFile) (bool, error) {
	changed := false
	for _, profile := range current.Profiles {
		baseline, applied := s.AppliedBindings[profile.ID]
		mapped, exists := s.Bindings[profile.ID]
		if !applied || !exists {
			continue
		}
		local := profileBinding(profile)
		if sameBinding(local, baseline) {
			continue
		}
		if !sameBinding(mapped, baseline) && !sameBinding(mapped, local) {
			return false, fmt.Errorf("local favorite and sync mapping both changed; choose the intended local mapping explicitly")
		}
		if err := validateBinding(local); err != nil {
			return false, err
		}
		local.Browser, _ = canonicalBrowser(local.Browser)
		if !sameBinding(mapped, local) {
			s.Bindings[profile.ID] = local
			changed = true
		}
		s.AppliedBindings[profile.ID] = local
		changed = true
	}
	return changed, nil
}

type graph struct {
	revisions map[string]revision
	heads     []string
}

func validateRevision(r revision) error {
	if r.Version != 1 || !revisionID.MatchString(r.ID) || !revisionID.MatchString(r.DeviceID) || r.CreatedAt.IsZero() || len(r.Parents) > MaxRevisions {
		return fmt.Errorf("invalid settings revision envelope")
	}
	seen := map[string]bool{}
	for _, parent := range r.Parents {
		if !revisionID.MatchString(parent) || parent == r.ID || seen[parent] {
			return fmt.Errorf("invalid revision parent")
		}
		seen[parent] = true
	}
	return validatePortable(r.Settings)
}

func (e *Engine) scan() (graph, error) {
	dir := filepath.Join(e.cloudDir, "revisions")
	names, err := cloudList(dir)
	if err != nil {
		return graph{}, err
	}
	if len(names) > MaxRevisions+16 {
		return graph{}, fmt.Errorf("settings revision directory exceeds scan limit")
	}
	g := graph{revisions: map[string]revision{}, heads: []string{}}
	for _, name := range names {
		if name == ".DS_Store" || strings.HasPrefix(name, ".settingssync-") {
			continue
		}
		if !strings.HasSuffix(name, ".json") || !revisionID.MatchString(strings.TrimSuffix(name, ".json")) {
			return g, fmt.Errorf("unexpected file in settings revision history")
		}
		data, err := cloudRead(filepath.Join(dir, name))
		if err != nil {
			return g, err
		}
		if len(data) > MaxRevisionBytes {
			return g, fmt.Errorf("oversized settings revision")
		}
		var r revision
		if err := strictJSON(data, &r); err != nil {
			return g, fmt.Errorf("malformed settings revision: %w", err)
		}
		if err := validateRevision(r); err != nil {
			return g, err
		}
		if r.ID+".json" != name {
			return g, fmt.Errorf("revision identity does not match its filename")
		}
		if _, exists := g.revisions[r.ID]; exists {
			return g, fmt.Errorf("duplicate settings revision identity")
		}
		g.revisions[r.ID] = r
	}
	if len(g.revisions) > MaxRevisions {
		return g, fmt.Errorf("settings revision limit exceeded")
	}
	parents := map[string]bool{}
	for _, r := range g.revisions {
		for _, parent := range r.Parents {
			if _, ok := g.revisions[parent]; !ok {
				return g, &cloudFailure{"pending", "Waiting for missing parent revisions; existing settings stay active"}
			}
			parents[parent] = true
		}
	}
	visiting := map[string]bool{}
	done := map[string]bool{}
	var visit func(string) error
	visit = func(id string) error {
		if visiting[id] {
			return fmt.Errorf("cycle in settings revision history")
		}
		if done[id] {
			return nil
		}
		visiting[id] = true
		for _, p := range g.revisions[id].Parents {
			if err := visit(p); err != nil {
				return err
			}
		}
		visiting[id] = false
		done[id] = true
		return nil
	}
	for id := range g.revisions {
		if err := visit(id); err != nil {
			return g, err
		}
		if !parents[id] {
			g.heads = append(g.heads, id)
		}
	}
	sort.Strings(g.heads)
	return g, nil
}

func checkApplied(s state, g graph) error {
	if s.Applied != "" {
		if applied, exists := g.revisions[s.Applied]; !exists {
			return &cloudFailure{"pending", "Previously applied revision is missing; existing settings stay active"}
		} else if s.Base == nil || !equalSettings(applied.Settings, *s.Base) {
			return fmt.Errorf("an already applied immutable revision changed contents; existing settings stay active")
		}
	}
	if len(g.heads) == 0 {
		return &cloudFailure{"pending", "Waiting for a complete settings revision history"}
	}
	return nil
}
