package learning

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"
)

var epoch = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	return NewStore(filepath.Join(t.TempDir(), "learning", "data.json"))
}
func observation(minute int, ids ...string) Observation {
	destinations := []Destination{}
	for _, id := range ids {
		destinations = append(destinations, Destination{ID: id})
	}
	at := epoch.Add(time.Duration(minute) * time.Minute)
	return Observation{Enabled: true, URL: "https://EXAMPLE.com/path?token=secret#title", SourceBundle: "com.apple.mail", SourcePrivacy: Public, ReceivedAt: at, ObservedAt: at.Add(time.Second), Destinations: destinations}
}
func observe(t *testing.T, s *Store, minute int, ids ...string) Request {
	t.Helper()
	r, err := s.Observe(observation(minute, ids...))
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func confirm(t *testing.T, s *Store, r Request, id string) {
	t.Helper()
	if err := s.Confirm(r.ID, id, r.ObservedAt.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
}
func predictRequest(t *testing.T, s *Store, r Request) Prediction {
	t.Helper()
	p, err := s.Predict(r.ID, r.ReceivedAt)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestNormalizationPrivacyAndOptIn(t *testing.T) {
	s := newTestStore(t)
	for _, raw := range []string{"https://person:secret@example.com/path", "file:///tmp/private", "http://localhost/a", "https://host.internal/a", "https://127.0.0.1/a", "https://127.1/a", "https://0x7f.0.0.1/a", "https://127.0x1/a", "https://[::1]/a", "http://intranet/a", "https://a.onion/", "not a url"} {
		input := observation(1, "work")
		input.URL = raw
		if _, err := s.Observe(input); !errors.Is(err, ErrExcluded) {
			t.Errorf("%q: %v", raw, err)
		}
	}
	for _, privacy := range []Privacy{Private, Unknown, ""} {
		input := observation(1, "work")
		input.SourcePrivacy = privacy
		if _, err := s.Observe(input); !errors.Is(err, ErrExcluded) {
			t.Fatal(err)
		}
	}
	input := observation(1, "work")
	input.Enabled = false
	if _, err := s.Observe(input); !errors.Is(err, ErrDisabled) {
		t.Fatal(err)
	}
	if _, err := os.Stat(s.path); !os.IsNotExist(err) {
		t.Fatal("excluded observations created storage")
	}
	input = observation(1, "work")
	input.Destinations = append(input.Destinations, Destination{ID: "private", Private: true})
	r, err := s.Observe(input)
	if err != nil {
		t.Fatal(err)
	}
	if r.Domain != "example.com" || !reflect.DeepEqual(r.DestinationIDs, []string{"work"}) {
		t.Fatalf("unexpected normalized request: %+v", r)
	}
	if err := s.Confirm(r.ID, "private", r.ObservedAt.Add(time.Second)); err == nil {
		t.Fatal("private destination label accepted")
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"token", "secret", "/path", "title", "https://"} {
		if bytes.Contains(data, []byte(secret)) {
			t.Errorf("persisted %q", secret)
		}
	}
}

func TestAutomaticObservationNeverCreatesLabel(t *testing.T) {
	s := newTestStore(t)
	observe(t, s, 1, "work")
	r := observe(t, s, 2, "work")
	data, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Feedback) != 0 {
		t.Fatal("implicit label created")
	}
	if p := predictRequest(t, s, r); p.AbstainReason != "no_confirmed_history" {
		t.Fatalf("%+v", p)
	}
	e, err := s.Evaluate(epoch.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if e.Observed != 2 || e.Labeled != 0 || e.Status != "data_not_ready" || e.ErrorRate != nil {
		t.Fatalf("%+v", e)
	}
}

func TestStrictTimeAndFutureInvariance(t *testing.T) {
	s := newTestStore(t)
	a := observe(t, s, 1, "work")
	confirm(t, s, a, "work")
	q := observe(t, s, 3, "work")
	before := predictRequest(t, s, q)
	if before.DestinationID != "work" || before.Support != 1 {
		t.Fatalf("%+v", before)
	}
	late := observe(t, s, 2, "work")
	if err := s.Confirm(late.ID, "work", epoch.Add(4*time.Minute)); err != nil {
		t.Fatal(err)
	}
	future := observe(t, s, 5, "work")
	confirm(t, s, future, "work")
	if err := s.Correct(a.ID, "work", epoch.Add(6*time.Minute)); err != nil {
		t.Fatal(err)
	}
	after := predictRequest(t, s, q)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("future data changed past prediction:\n%+v\n%+v", before, after)
	}
	if p, err := s.Predict(q.ID, epoch.Add(10*time.Minute)); err != nil || !reflect.DeepEqual(p, before) {
		t.Fatal("later cutoff leaked future data", p, err)
	}
	if q.CapturedPrediction == nil || !reflect.DeepEqual(*q.CapturedPrediction, before) {
		t.Fatal("capture differs from initial replay")
	}
}

func TestCorrectionsAreVersionedAndAsOf(t *testing.T) {
	s := newTestStore(t)
	a := observe(t, s, 1, "personal", "work")
	confirm(t, s, a, "work")
	b := observe(t, s, 2, "personal", "work")
	confirm(t, s, b, "personal")
	q := observe(t, s, 3, "personal", "work")
	before := predictRequest(t, s, q)
	if before.DestinationID != "personal" {
		t.Fatalf("recency tie break failed %+v", before)
	}
	if err := s.Correct(b.ID, "work", epoch.Add(4*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, predictRequest(t, s, q)) {
		t.Fatal("correction rewrote historical prediction")
	}
	// Historical support for personal disappears once its only explicit label
	// is corrected. The now-unsupported candidate requires abstention.
	later := observe(t, s, 5, "work", "personal")
	if p := predictRequest(t, s, later); p.AbstainReason != "new_or_unsupported_destination" {
		t.Fatalf("%+v", p)
	}
	data, _ := s.Snapshot()
	if len(data.Feedback) != 3 || data.Feedback[2].Version != 2 || data.Feedback[2].Kind != "explicit_correction" {
		t.Fatal(data.Feedback)
	}
	if err := s.Confirm(a.ID, "work", epoch.Add(6*time.Minute)); err == nil {
		t.Fatal("duplicate confirmation accepted")
	}
	if err := s.Correct(q.ID, "work", epoch.Add(6*time.Minute)); err == nil {
		t.Fatal("unconfirmed correction accepted")
	}
	if err := s.Correct(b.ID, "personal", epoch.Add(4*time.Minute)); err == nil {
		t.Fatal("same-time correction accepted")
	}
}

func TestCurrentRequestAndEqualTimeAreExcluded(t *testing.T) {
	s := newTestStore(t)
	a := observe(t, s, 1, "work")
	confirm(t, s, a, "work")
	input := observation(2, "work")
	input.ReceivedAt = a.ObservedAt.Add(time.Second)
	input.ObservedAt = input.ReceivedAt.Add(time.Second)
	q, err := s.Observe(input)
	if err != nil {
		t.Fatal(err)
	}
	confirm(t, s, q, "work")
	if p := predictRequest(t, s, q); p.AbstainReason != "no_confirmed_history" {
		t.Fatalf("equal-time confirmation or current label leaked: %+v", p)
	}
}

func TestBaselineSourceFallbackAndUnavailableProfiles(t *testing.T) {
	s := newTestStore(t)
	a := observe(t, s, 1, "work", "personal")
	confirm(t, s, a, "work")
	b := observe(t, s, 2, "work", "personal")
	confirm(t, s, b, "personal")
	c := observe(t, s, 3, "work", "personal")
	confirm(t, s, c, "work")
	input := observation(4, "work", "personal")
	input.SourceBundle = "com.apple.notes"
	q, err := s.Observe(input)
	if err != nil {
		t.Fatal(err)
	}
	if p := predictRequest(t, s, q); p.Method != "domain" || p.DestinationID != "work" || p.Support != 2 || p.MatchingLabels != 3 {
		t.Fatalf("%+v", p)
	}
	removed := observe(t, s, 5, "personal")
	if p := predictRequest(t, s, removed); p.AbstainReason != "historical_destination_unavailable" || p.DestinationID != "" {
		t.Fatalf("unavailable winner silently reassigned: %+v", p)
	}
	added := observe(t, s, 6, "work", "personal", "academic")
	if p := predictRequest(t, s, added); p.AbstainReason != "new_or_unsupported_destination" {
		t.Fatalf("%+v", p)
	}
	input = observation(7, "work", "personal")
	input.URL = "https://unseen.example.org"
	unseen, err := s.Observe(input)
	if err != nil {
		t.Fatal(err)
	}
	if p := predictRequest(t, s, unseen); p.AbstainReason != "unseen_domain" {
		t.Fatalf("%+v", p)
	}
}

func TestEvaluationUsesFirstNeutralTruthAndHistoricalCorrections(t *testing.T) {
	s := newTestStore(t)
	a := observe(t, s, 1, "work", "personal")
	confirm(t, s, a, "work")
	b := observe(t, s, 2, "work", "personal")
	confirm(t, s, b, "personal")
	c := observe(t, s, 3, "work", "personal")
	confirm(t, s, c, "personal")
	before, err := s.Evaluate(epoch.Add(4 * time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if before.Labeled != 3 || before.Predicted != 1 || before.Correct != 1 || before.Errors != 0 || before.Abstained != 2 {
		t.Fatalf("%+v", before)
	}
	if err := s.Correct(c.ID, "work", epoch.Add(5*time.Minute)); err != nil {
		t.Fatal(err)
	}
	old, _ := s.Evaluate(epoch.Add(4 * time.Minute))
	if !reflect.DeepEqual(old, before) {
		t.Fatal("future correction changed past evaluation")
	}
	after, _ := s.Evaluate(epoch.Add(6 * time.Minute))
	if after.Correct != 1 || after.CorrectedLabels != 1 || after.NeutralLabels != 3 {
		t.Fatalf("correction silently rewrote outcome: %+v", after)
	}
	if after.Predicted+after.Abstained != after.Labeled || after.ErrorRate == nil || *after.ErrorRate != 0 {
		t.Fatalf("bad denominators %+v", after)
	}
}

func TestDeletionEquivalentToCleanRebuild(t *testing.T) {
	s := newTestStore(t)
	a := observe(t, s, 1, "work", "personal")
	confirm(t, s, a, "work")
	b := observe(t, s, 2, "work", "personal")
	confirm(t, s, b, "personal")
	c := observe(t, s, 3, "work", "personal")
	confirm(t, s, c, "work")
	if err := s.Correct(c.ID, "personal", epoch.Add(4*time.Minute)); err != nil {
		t.Fatal(err)
	}
	q := observe(t, s, 5, "work", "personal")
	if err := s.DeleteDestination("personal"); err != nil {
		t.Fatal(err)
	}
	data, _ := s.Snapshot()
	if len(data.Requests) != 2 || len(data.Feedback) != 1 {
		t.Fatalf("deleted profile revived older label: %+v", data)
	}
	for _, r := range data.Requests {
		if r.CapturedPrediction != nil || contains(r.DestinationIDs, "personal") {
			t.Fatal("derived/deleted data retained")
		}
	}
	fresh := newTestStore(t)
	if err := fresh.write(data); err != nil {
		t.Fatal(err)
	}
	left := predictRequest(t, s, q)
	right := predictRequest(t, fresh, q)
	if !reflect.DeepEqual(left, right) || left.DestinationID != "work" {
		t.Fatalf("delete/rebuild mismatch %+v %+v", left, right)
	}
	if err := s.DeleteRequest(a.ID); err != nil {
		t.Fatal(err)
	}
	if p := predictRequest(t, s, q); p.AbstainReason != "no_confirmed_history" {
		t.Fatal("deleted history survived", p)
	}
	if err := s.Clear(); err != nil {
		t.Fatal(err)
	}
	data, _ = s.Snapshot()
	if len(data.Requests) != 0 || len(data.Feedback) != 0 {
		t.Fatal(data)
	}
}

func TestStoragePermissionsCorruptionAndAtomicReplacement(t *testing.T) {
	s := newTestStore(t)
	observe(t, s, 1, "work")
	for _, entry := range []struct {
		path string
		mode os.FileMode
	}{{s.path, 0600}, {filepath.Dir(s.path), 0700}} {
		info, err := os.Stat(entry.path)
		if err != nil || info.Mode().Perm() != entry.mode {
			t.Fatalf("permissions %v %v", info, err)
		}
	}
	original, _ := os.ReadFile(s.path)
	if err := os.WriteFile(s.path, []byte("{broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Observe(observation(2, "work")); err == nil {
		t.Fatal("overwrote corrupt store")
	}
	broken, _ := os.ReadFile(s.path)
	if string(broken) != "{broken" {
		t.Fatal("corruption not preserved")
	}
	if err := s.Clear(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.path, append(original, []byte(" {}")...), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Snapshot(); err == nil {
		t.Fatal("accepted trailing data")
	}
	if err := s.Clear(); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	failures := make(chan error, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := NewStore(s.path).Observe(observation(i+1, "work"))
			failures <- err
		}(i)
	}
	wg.Wait()
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	data, err := s.Snapshot()
	if err != nil || len(data.Requests) != 12 {
		t.Fatal(data, err)
	}
	files, _ := os.ReadDir(filepath.Dir(s.path))
	if len(files) != 1 || files[0].Name() != "data.json" {
		t.Fatal("temporary writes retained", files)
	}
}

func TestRejectSymlinkAndOversizedStore(t *testing.T) {
	s := newTestStore(t)
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "other")
	if err := os.WriteFile(target, []byte("untouched"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, s.path); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Snapshot(); err == nil {
		t.Fatal("read symlink")
	}
	if err := s.Clear(); err == nil {
		t.Fatal("wrote symlink")
	}
	content, _ := os.ReadFile(target)
	if string(content) != "untouched" {
		t.Fatal("modified symlink target")
	}
	if err := os.Remove(s.path); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(s.path)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.Truncate(MaxFileBytes + 1); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if _, err := s.Snapshot(); err == nil {
		t.Fatal("accepted oversized store")
	}
}

func TestRetentionDropsWholeOldestRequestsAndFeedback(t *testing.T) {
	s := newTestStore(t)
	data := emptySnapshot()
	for i := 0; i < RequestLimit; i++ {
		// Build deterministic unique valid IDs without incurring thousands of disk writes.
		id := fmtID(i)
		r := Request{ID: id, ReceivedAt: epoch.Add(time.Duration(i) * time.Second), ObservedAt: epoch.Add(time.Duration(i) * time.Second), Domain: "example.com", SourceBundle: "com.apple.mail", SourcePrivacy: Public, DestinationIDs: []string{"work"}}
		data.Requests = append(data.Requests, r)
		data.Feedback = append(data.Feedback, Feedback{id, 1, "work", "neutral_chooser", r.ObservedAt})
	}
	oldest := data.Requests[0].ID
	if err := s.write(data); err != nil {
		t.Fatal(err)
	}
	observe(t, s, RequestLimit, "work")
	data, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Requests) != RequestLimit || len(data.Feedback) != RequestLimit-1 {
		t.Fatalf("retention counts %d %d", len(data.Requests), len(data.Feedback))
	}
	for _, r := range data.Requests {
		if r.ID == oldest {
			t.Fatal("oldest request retained")
		}
	}
	for _, f := range data.Feedback {
		if f.RequestID == oldest {
			t.Fatal("orphan feedback retained")
		}
	}
}

func fmtID(i int) string {
	bytes := make([]byte, 16)
	for j := 15; j >= 0; j-- {
		bytes[j] = byte(i)
		i >>= 8
	}
	const digits = "0123456789abcdef"
	out := make([]byte, 32)
	for j, b := range bytes {
		out[j*2] = digits[b>>4]
		out[j*2+1] = digits[b&15]
	}
	return string(out)
}

func TestRejectInvalidPersistedProvenance(t *testing.T) {
	s := newTestStore(t)
	r := observe(t, s, 1, "work")
	confirm(t, s, r, "work")
	data, _ := s.Snapshot()
	data.Feedback[0].Kind = "automatic"
	encoded, _ := json.Marshal(data)
	os.WriteFile(s.path, encoded, 0600)
	if _, err := s.Snapshot(); err == nil {
		t.Fatal("accepted inferred label")
	}
}

func TestSourceSpecificHistoryPrecedesDomainMajority(t *testing.T) {
	s := newTestStore(t)
	for i := 1; i <= 3; i++ {
		r := observe(t, s, i, "work", "personal")
		confirm(t, s, r, "work")
	}
	input := observation(4, "work", "personal")
	input.SourceBundle = "com.apple.notes"
	r, err := s.Observe(input)
	if err != nil {
		t.Fatal(err)
	}
	confirm(t, s, r, "personal")
	input = observation(5, "work", "personal")
	input.SourceBundle = "com.apple.notes"
	q, err := s.Observe(input)
	if err != nil {
		t.Fatal(err)
	}
	p := predictRequest(t, s, q)
	if p.Method != "domain_and_source" || p.DestinationID != "personal" || p.Support != 1 {
		t.Fatalf("%+v", p)
	}
}

func TestTiesUseStableDestinationIdentity(t *testing.T) {
	s := newTestStore(t)
	a := observe(t, s, 1, "aaa", "zzz")
	b := observe(t, s, 2, "aaa", "zzz")
	at := epoch.Add(3 * time.Minute)
	if err := s.Confirm(a.ID, "zzz", at); err != nil {
		t.Fatal(err)
	}
	if err := s.Confirm(b.ID, "aaa", at); err != nil {
		t.Fatal(err)
	}
	q := observe(t, s, 4, "aaa", "zzz")
	if p := predictRequest(t, s, q); p.DestinationID != "aaa" {
		t.Fatalf("%+v", p)
	}
}

func TestFeedbackRetentionKeepsWholeVersionHistories(t *testing.T) {
	data := emptySnapshot()
	for i := 0; i < 3; i++ {
		r := Request{ID: fmtID(i), ReceivedAt: epoch.Add(time.Duration(i) * time.Second), ObservedAt: epoch.Add(time.Duration(i) * time.Second), Domain: "example.com", SourceBundle: "com.apple.mail", SourcePrivacy: Public, DestinationIDs: []string{"work"}}
		data.Requests = append(data.Requests, r)
		for version := 1; version <= 3000; version++ {
			kind := "explicit_correction"
			if version == 1 {
				kind = "neutral_chooser"
			}
			data.Feedback = append(data.Feedback, Feedback{r.ID, version, "work", kind, r.ObservedAt.Add(time.Duration(version) * time.Second)})
		}
	}
	trim(&data)
	if len(data.Requests) != 2 || len(data.Feedback) != 6000 {
		t.Fatalf("retention split a version history: %d %d", len(data.Requests), len(data.Feedback))
	}
	if err := validate(data); err != nil {
		t.Fatal(err)
	}
}

func TestInvalidWriteDoesNotReplaceValidFile(t *testing.T) {
	s := newTestStore(t)
	observe(t, s, 1, "work")
	before, _ := os.ReadFile(s.path)
	data, _ := s.Snapshot()
	data.Requests[0].CapturedPrediction.Method = "https://private.example/path"
	if err := s.write(data); err == nil {
		t.Fatal("invalid capture accepted")
	}
	after, _ := os.ReadFile(s.path)
	if !bytes.Equal(before, after) {
		t.Fatal("invalid write replaced valid data")
	}
}

func TestRetainDestinationsRemovesLifecycleEvidenceAtomically(t *testing.T) {
	s := newTestStore(t)
	ids := []string{"kept", "removed", "hidden", "now-private", "old-target"}
	kept := observe(t, s, 1, ids...)
	confirm(t, s, kept, "kept")
	for i, id := range ids[1:] {
		r := observe(t, s, i+2, ids...)
		confirm(t, s, r, id)
	}
	corrected := observe(t, s, 6, ids...)
	confirm(t, s, corrected, "removed")
	if err := s.Correct(corrected.ID, "kept", epoch.Add(7*time.Minute)); err != nil {
		t.Fatal(err)
	}
	unlabeled := observe(t, s, 8, ids...)
	changed, err := s.RetainDestinations([]string{"kept", "new-target"})
	if err != nil || !changed {
		t.Fatal(changed, err)
	}
	data, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Requests) != 2 || len(data.Feedback) != 1 {
		t.Fatalf("obsolete evidence retained %+v", data)
	}
	for _, r := range data.Requests {
		if r.ID != kept.ID && r.ID != unlabeled.ID {
			t.Fatal("obsolete request retained", r.ID)
		}
		if !reflect.DeepEqual(r.DestinationIDs, []string{"kept"}) || r.CapturedPrediction != nil {
			t.Fatalf("candidate/capture not purged %+v", r)
		}
	}
	if p := predictRequest(t, s, unlabeled); p.DestinationID != "kept" || p.Support != 1 {
		t.Fatalf("obsolete correction survived %+v", p)
	}
	if err := s.Confirm(corrected.ID, "kept", epoch.Add(9*time.Minute)); !errors.Is(err, ErrNotFound) {
		t.Fatal("removed request resurrected", err)
	}
	fresh := newTestStore(t)
	if err := fresh.write(data); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(predictRequest(t, s, unlabeled), predictRequest(t, fresh, unlabeled)) {
		t.Fatal("reconciliation differs from clean rebuild")
	}
	changed, err = s.RetainDestinations(nil)
	if err != nil || !changed {
		t.Fatal(changed, err)
	}
	data, _ = s.Snapshot()
	if len(data.Requests) != 0 || len(data.Feedback) != 0 {
		t.Fatal("empty destination set retained data")
	}
}

func TestRetainDestinationsPreservesRenameAndDoesNotRewrite(t *testing.T) {
	s := newTestStore(t)
	stableID := "com.google.Chrome|Profile 1"
	r := observe(t, s, 1, stableID)
	confirm(t, s, r, stableID)
	q := observe(t, s, 2, stableID)
	before, _ := os.ReadFile(s.path)
	oldInfo, _ := os.Stat(s.path)
	// A display rename is represented by exactly the same browser/profile
	// directory identity. Duplicate or reordered current IDs are harmless.
	changed, err := s.RetainDestinations([]string{stableID, stableID})
	if err != nil || changed {
		t.Fatal(changed, err)
	}
	after, _ := os.ReadFile(s.path)
	newInfo, _ := os.Stat(s.path)
	if !bytes.Equal(before, after) || !os.SameFile(oldInfo, newInfo) || !oldInfo.ModTime().Equal(newInfo.ModTime()) {
		t.Fatal("no-op reconciliation rewrote store")
	}
	data, _ := s.Snapshot()
	retained, _ := findRequest(data, q.ID)
	if retained.CapturedPrediction == nil || retained.CapturedPrediction.DestinationID != stableID {
		t.Fatal("rename erased capture")
	}
	if changed, err := s.RetainDestinations([]string{"/raw/profile/path"}); err == nil || changed {
		t.Fatal("invalid identity accepted")
	}
	final, _ := os.ReadFile(s.path)
	if !bytes.Equal(before, final) {
		t.Fatal("invalid input changed data")
	}
	empty := newTestStore(t)
	if changed, err := empty.RetainDestinations(nil); err != nil || changed {
		t.Fatal(changed, err)
	}
	if _, err := os.Stat(empty.path); !os.IsNotExist(err) {
		t.Fatal("empty reconciliation created a file")
	}
}

func TestRetentionClearsDerivedCapturesAndCapturesAgainstRemainingData(t *testing.T) {
	s := newTestStore(t)
	data := emptySnapshot()
	for i := 0; i < RequestLimit; i++ {
		at := epoch.Add(time.Duration(i) * time.Second)
		r := Request{ID: fmtID(i), ReceivedAt: at, ObservedAt: at, Domain: "example.com", SourceBundle: "com.apple.mail", SourcePrivacy: Public, DestinationIDs: []string{"work"}}
		if i > 0 {
			r.CapturedPrediction = &Prediction{RequestID: r.ID, DestinationID: "work", Method: "domain_and_source", Support: 1, MatchingLabels: 1, Provenance: []Evidence{{RequestID: fmtID(0), FeedbackVersion: 1, Kind: "neutral_chooser", AvailableAt: epoch}}}
		}
		data.Requests = append(data.Requests, r)
	}
	data.Feedback = append(data.Feedback, Feedback{fmtID(0), 1, "work", "neutral_chooser", epoch})
	if err := s.write(data); err != nil {
		t.Fatal(err)
	}
	newest := observe(t, s, RequestLimit, "work")
	data, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range data.Requests {
		if r.ID == newest.ID {
			if r.CapturedPrediction == nil || r.CapturedPrediction.AbstainReason != "no_confirmed_history" {
				t.Fatal("new capture used evicted label", r.CapturedPrediction)
			}
		} else if r.CapturedPrediction != nil {
			t.Fatal("retained derived capture references evicted label")
		}
	}
	if len(data.Feedback) != 0 {
		t.Fatal("evicted label survived")
	}
}
