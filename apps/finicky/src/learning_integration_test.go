package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"finicky/browser"
	"finicky/learning"
	"finicky/resolver"
	"finicky/rules"
	"finicky/urlrequest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func integrationLearningFixture(t *testing.T) (*learning.Store, string, URLInfo, rules.RulesFile, browser.BrowserConfig) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "learning", "observations.json")
	target := browser.BrowserConfig{Name: "Google Chrome", AppType: "Chromium", Profile: "Work", ProfileDirectory: "Profile 1", URL: "https://example.com/transformed-secret"}
	info := URLInfo{
		ReceivedAt: time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC),
		Request:    urlrequest.Request{Action: "open", URL: "https://EXAMPLE.com/private-path?token=secret-query#secret-fragment", SourcePrivacy: "public"},
		Opener:     &resolver.OpenerInfo{Name: "Google Chrome", BundleID: "com.google.Chrome", WindowTitle: "secret-window-title"},
	}
	rf := rules.RulesFile{
		DefaultBrowser: "Google Chrome", DefaultProfileDirectory: "Default",
		Rules:    []rules.Rule{{Name: "Unrelated routing rule", Match: []string{"https://other.example/*"}, Browser: "Safari"}},
		Features: &rules.Features{LearningEnabled: true, PromptMode: "never", StripTracking: true},
		Profiles: []rules.SavedProfile{
			{ID: "work", Name: "Work", Browser: "Google Chrome", Profile: "Work", ProfileDirectory: "Profile 1"},
			{ID: "personal", Name: "Personal", Browser: "Google Chrome", Profile: "Personal", ProfileDirectory: "Default"},
		},
	}
	return learning.NewStore(path), path, info, rf, target
}

func TestLearningIdentitySurvivesProfileRenamesAndBrowserAliases(t *testing.T) {
	original := browser.BrowserConfig{Name: "Google Chrome", Profile: "Old name", ProfileDirectory: "Profile 1"}
	want := learningTargetID(original)
	if len(want) != 64 {
		t.Fatalf("expected a stable opaque identity, got %q", want)
	}
	for _, name := range []string{"Google Chrome", "com.google.Chrome", "/Applications/Google Chrome.app"} {
		changed := original
		changed.Name, changed.Profile, changed.URL, changed.NewWindow = name, "Renamed", "https://example.com/another", true
		if got := learningTargetID(changed); got != want {
			t.Fatalf("rename, alias or launch-only flags changed identity: %q != %q", got, want)
		}
	}
	other := original
	other.ProfileDirectory = "Profile 2"
	if learningTargetID(other) == want {
		t.Fatal("distinct profile directories share an identity")
	}
	for _, excluded := range []browser.BrowserConfig{
		{Name: "Google Chrome", Profile: "Display name without directory"},
		{Name: "Google Chrome", ProfileDirectory: "Profile 1", Private: true},
		{Name: "Google Chrome", ProfileDirectory: "Profile 1", Args: []string{"--incognito"}},
		{Name: "Google Chrome", ProfileDirectory: "Profile 1", Args: []string{"--user-data-dir=/other-profile"}},
		{Name: "prompt"}, {},
	} {
		if learningTargetID(excluded) != "" {
			t.Fatalf("nonconcrete or private destination got identity: %+v", excluded)
		}
	}
}

func TestLearningDestinationsExcludePrivateHiddenAndDuplicateFavorites(t *testing.T) {
	_, _, _, rf, target := integrationLearningFixture(t)
	rf.Profiles = append(rf.Profiles,
		rules.SavedProfile{ID: "work-renamed", Name: "Duplicate label", Browser: "com.google.Chrome", Profile: "Renamed", ProfileDirectory: "Profile 1"},
		rules.SavedProfile{ID: "private", Name: "Private", Browser: "Google Chrome", ProfileDirectory: "Profile 9", Private: true},
		rules.SavedProfile{ID: "hidden", Name: "Hidden", Browser: "Google Chrome", ProfileDirectory: "Profile 8", Hidden: true},
		rules.SavedProfile{ID: "unstable", Name: "Unstable", Browser: "Google Chrome", Profile: "Name only"},
	)
	got := learningDestinations(rf)
	if len(got) != 2 || got[0].ID != learningTargetID(target) || got[0].Name != "Work" || got[1].Name != "Personal" {
		t.Fatalf("unexpected learnable favorites: %+v", got)
	}
	if got := learningDestinations(rules.RulesFile{}); got == nil || len(got) != 0 {
		t.Fatalf("empty destinations must serialize as [], got %#v", got)
	}
	rf.Profiles = append(rf.Profiles, rules.SavedProfile{ID: "hidden-duplicate", Name: "Hidden Work", Browser: "Google Chrome", ProfileDirectory: "Profile 1", Hidden: true})
	if got := learningDestinations(rf); len(got) != 1 || got[0].Name != "Personal" {
		t.Fatalf("hidden duplicate did not suppress its regular identity: %+v", got)
	}
}

func TestLearningSourceRequiresExplicitPublicMetadataAndEligibleBrowser(t *testing.T) {
	_, _, base, _, _ := integrationLearningFixture(t)
	for _, bundle := range []string{"com.google.Chrome", "com.google.Chrome.beta", "org.mozilla.firefox", "com.brave.Browser", "com.microsoft.edgemac", "com.vivaldi.Vivaldi"} {
		info := base
		info.Opener = &resolver.OpenerInfo{BundleID: bundle}
		if !eligibleLearningSource(info) {
			t.Errorf("explicit regular browser source rejected: %q", bundle)
		}
	}
	for _, privacy := range []string{"", "unknown", "private", "PUBLIC", "false"} {
		info := base
		info.Request.SourcePrivacy = privacy
		if eligibleLearningSource(info) {
			t.Errorf("privacy %q was considered public", privacy)
		}
	}
	for _, opener := range []*resolver.OpenerInfo{nil, {}, {BundleID: "com.apple.Terminal"}, {BundleID: "com.apple.shortcuts"}, {BundleID: "com.apple.Safari"}, {BundleID: "com.google.Chrome.evil"}} {
		info := base
		info.Opener = opener
		if eligibleLearningSource(info) {
			t.Errorf("unverified sender was eligible: %+v", opener)
		}
	}
}

func TestRecordLearningExclusionsNeverCreateStorage(t *testing.T) {
	cases := []struct {
		name   string
		change func(*URLInfo, *rules.RulesFile, *browser.BrowserConfig)
	}{
		{"unknown source", func(i *URLInfo, _ *rules.RulesFile, _ *browser.BrowserConfig) { i.Request.SourcePrivacy = "unknown" }},
		{"private source", func(i *URLInfo, _ *rules.RulesFile, _ *browser.BrowserConfig) { i.Request.SourcePrivacy = "private" }},
		{"absent opener", func(i *URLInfo, _ *rules.RulesFile, _ *browser.BrowserConfig) { i.Opener = nil }},
		{"unrelated sender", func(i *URLInfo, _ *rules.RulesFile, _ *browser.BrowserConfig) {
			i.Opener = &resolver.OpenerInfo{BundleID: "com.apple.Terminal"}
		}},
		{"private request", func(i *URLInfo, _ *rules.RulesFile, _ *browser.BrowserConfig) { i.Request.Private = true }},
		{"private final destination", func(_ *URLInfo, _ *rules.RulesFile, b *browser.BrowserConfig) { b.Private = true }},
		{"custom private launch arguments", func(_ *URLInfo, _ *rules.RulesFile, b *browser.BrowserConfig) { b.Args = []string{"--incognito"} }},
		{"custom profile launch arguments", func(_ *URLInfo, _ *rules.RulesFile, b *browser.BrowserConfig) {
			b.Args = []string{"--user-data-dir=/other-profile"}
		}},
		{"matched rule", func(_ *URLInfo, _ *rules.RulesFile, b *browser.BrowserConfig) { index := 0; b.MatchedRule = &index }},
		{"explicit browser", func(i *URLInfo, _ *rules.RulesFile, _ *browser.BrowserConfig) { i.Request.Browser = "Google Chrome" }},
		{"disabled", func(_ *URLInfo, r *rules.RulesFile, _ *browser.BrowserConfig) { r.Features.LearningEnabled = false }},
		{"absent features", func(_ *URLInfo, r *rules.RulesFile, _ *browser.BrowserConfig) { r.Features = nil }},
		{"unsaved target", func(_ *URLInfo, _ *rules.RulesFile, b *browser.BrowserConfig) { b.ProfileDirectory = "Profile 99" }},
		{"hidden target", func(_ *URLInfo, r *rules.RulesFile, _ *browser.BrowserConfig) { r.Profiles[0].Hidden = true }},
		{"credentials in URL", func(i *URLInfo, _ *rules.RulesFile, _ *browser.BrowserConfig) {
			i.Request.URL = "https://name:password@example.com"
		}},
		{"internal destination", func(i *URLInfo, _ *rules.RulesFile, _ *browser.BrowserConfig) {
			i.Request.URL = "https://project.internal/private"
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store, path, info, rf, target := integrationLearningFixture(t)
			tc.change(&info, &rf, &target)
			for _, neutral := range []bool{false, true} {
				err := recordLearningInStore(store, info, rf, target, neutral, info.ReceivedAt.Add(time.Second))
				if !errors.Is(err, learning.ErrExcluded) {
					t.Fatalf("expected exclusion, got %v", err)
				}
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatalf("excluded request wrote storage: %v", err)
			}
		})
	}
}

func TestRecordingDoesNotChangeRoutingAndAutomaticChoicesAreUnlabeled(t *testing.T) {
	store, path, info, rf, target := integrationLearningFixture(t)
	beforeRules, _ := json.Marshal(rf)
	beforeTarget, _ := json.Marshal(target)
	beforeInfo, _ := json.Marshal(info)
	if err := recordLearningInStore(store, info, rf, target, false, info.ReceivedAt.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	data, err := store.Snapshot()
	if err != nil || len(data.Requests) != 1 || len(data.Feedback) != 0 {
		t.Fatalf("automatic route must create no label: %+v, %v", data, err)
	}
	if data.Requests[0].Domain != "example.com" || data.Requests[0].SourceBundle != "com.google.Chrome" {
		t.Fatalf("unexpected sanitized request: %+v", data.Requests[0])
	}
	if data.Requests[0].CapturedPrediction == nil || data.Requests[0].CapturedPrediction.DestinationID != "" {
		t.Fatal("unconfirmed first route must abstain")
	}
	for _, pair := range []struct {
		before []byte
		after  interface{}
	}{{beforeRules, rf}, {beforeTarget, target}, {beforeInfo, info}} {
		after, _ := json.Marshal(pair.after)
		if !bytes.Equal(pair.before, after) {
			t.Fatalf("recording mutated routing inputs: %s -> %s", pair.before, after)
		}
	}
	stored, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	assertNoLearningURLSecrets(t, stored)
}

func assertNoLearningURLSecrets(t *testing.T, serialized []byte) {
	t.Helper()
	for _, secret := range []string{"private-path", "secret-query", "secret-fragment", "secret-window-title", "transformed-secret", "https://", "?token="} {
		if bytes.Contains(serialized, []byte(secret)) {
			t.Errorf("learning output contains URL or context data %q", secret)
		}
	}
}

func TestNeutralChooserLabelsAndSnapshotContract(t *testing.T) {
	store, _, info, rf, target := integrationLearningFixture(t)
	empty, err := learningSnapshot(store, rules.RulesFile{}, info.ReceivedAt)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(empty)
	var decoded map[string]interface{}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"observations", "destinations", "limitations"} {
		if _, ok := decoded[key].([]interface{}); !ok {
			t.Fatalf("%s is not a JSON array: %s", key, encoded)
		}
	}
	if empty["enabled"] != false || empty["feedbackCount"] != 0 {
		t.Fatalf("unexpected empty state: %+v", empty)
	}
	observedAt := info.ReceivedAt.Add(time.Second)
	if err := recordLearningInStore(store, info, rf, target, true, observedAt); err != nil {
		t.Fatal(err)
	}
	data, err := store.Snapshot()
	if err != nil || len(data.Requests) != 1 || len(data.Feedback) != 1 {
		t.Fatalf("%+v %v", data, err)
	}
	label := data.Feedback[0]
	if label.Kind != "neutral_chooser" || label.Version != 1 || label.DestinationID != learningTargetID(target) || !label.AvailableAt.After(observedAt) {
		t.Fatalf("chooser label lacks explicit provenance: %+v", label)
	}
	at := observedAt.Add(time.Second)
	snapshot, err := learningSnapshot(store, rf, at)
	if err != nil {
		t.Fatal(err)
	}
	rows := snapshot["observations"].([]map[string]interface{})
	if len(rows) != 1 || rows[0]["domain"] != "example.com" || rows[0]["confirmedDestinationName"] != "Work" || rows[0]["feedbackKind"] != "neutral_chooser" {
		t.Fatalf("unexpected chooser row: %+v", rows)
	}
	evaluation := snapshot["evaluation"].(map[string]int)
	if snapshot["feedbackCount"] != 1 || evaluation["evaluated"] != 1 || evaluation["predicted"] != 0 || evaluation["abstained"] != 1 || evaluation["correct"] != 0 {
		t.Fatalf("first label was treated as its own evidence: %+v", snapshot)
	}
	if evaluation["predicted"]+evaluation["abstained"] != evaluation["evaluated"] || evaluation["correct"] > evaluation["predicted"] {
		t.Fatal("inconsistent evaluation counters")
	}
	encoded, _ = json.Marshal(snapshot)
	assertNoLearningURLSecrets(t, encoded)

	// A later correction changes the displayed confirmed choice, while the
	// historical score continues to use the original neutral-chooser outcome.
	other := learningDestinations(rf)[1]
	if err := store.Correct(data.Requests[0].ID, other.ID, at.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	historical, err := learningSnapshot(store, rf, at)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(historical["observations"], snapshot["observations"]) {
		t.Fatal("future correction leaked into historical displayed confirmation")
	}
	current, err := learningSnapshot(store, rf, at.Add(2*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	currentRows := current["observations"].([]map[string]interface{})
	if currentRows[0]["confirmedDestinationName"] != "Personal" || currentRows[0]["feedbackKind"] != "explicit_correction" || current["feedbackCount"] != 1 {
		t.Fatalf("unexpected correction snapshot: %+v", current)
	}
}
