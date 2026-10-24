package main

import (
	"errors"
	"finicky/browser"
	"finicky/config"
	"finicky/picker"
	"finicky/rules"
	"finicky/urlrequest"
	"os"
	"path/filepath"
	"testing"
)

func TestDestinationKeyUsesStableDirectory(t *testing.T) {
	a := browser.BrowserConfig{Name: "Google Chrome", Profile: "Old name", ProfileDirectory: "Profile 2"}
	b := browser.BrowserConfig{Name: "com.google.Chrome", Profile: "New name", ProfileDirectory: "Profile 2"}
	if destinationKey(a) != destinationKey(b) {
		t.Fatal("renamed or bundle-ID target was not deduplicated")
	}
	b.Private = true
	if destinationKey(a) == destinationKey(b) {
		t.Fatal("private target collided with regular target")
	}
}

func TestFocusDefaultRestoresAcrossRepeatedActivation(t *testing.T) {
	rules.SetCustomPath(filepath.Join(t.TempDir(), "rules.json"))
	t.Cleanup(func() { rules.SetCustomPath("") })
	rf := rules.RulesFile{DefaultBrowser: "Google Chrome", DefaultProfileDirectory: "Default"}
	for _, dir := range []string{"Profile 1", "Profile 2"} {
		if err := applyDefaultRequest(&rf, urlrequest.Request{Focus: "active", Browser: "Google Chrome", ProfileDirectory: dir}); err != nil {
			t.Fatal(err)
		}
	}
	if rf.DefaultProfileDirectory != "Profile 2" {
		t.Fatal("focus target was not applied")
	}
	if err := applyDefaultRequest(&rf, urlrequest.Request{Focus: "clear"}); err != nil {
		t.Fatal(err)
	}
	if rf.DefaultProfileDirectory != "Default" {
		t.Fatal("original default was lost on repeated activation")
	}
}

func TestNameFavoriteUsesOnlyUnambiguousDiscoveredProfile(t *testing.T) {
	target := browser.BrowserConfig{Name: "Google Chrome", Profile: "Work"}
	profiles := []browser.BrowserProfile{{Name: "Work", Profile: "Work", ProfileDirectory: "Profile 2"}}
	if got := stableFavorite(target, profiles); got.ProfileDirectory != "Profile 2" {
		t.Fatalf("%+v", got)
	}
	profiles = append(profiles, browser.BrowserProfile{Name: "Work", Profile: "Work", ProfileDirectory: "Profile 3"})
	if got := stableFavorite(target, profiles); got.ProfileDirectory != "" {
		t.Fatal("ambiguous display name selected a directory")
	}
}

func TestPickerPreservesRequiredModesAndChosenProfile(t *testing.T) {
	background := true
	rule := 0
	resolved := browser.BrowserConfig{
		Name: "Google Chrome", ProfileDirectory: "Profile 1", URL: "https://example.com/private",
		Private: true, NewWindow: true, OpenInBackground: &background, MatchedRule: &rule,
		Args: []string{"--browser-specific-flag"},
	}
	chosen := browser.BrowserConfig{Name: "Microsoft Edge", ProfileDirectory: "Profile 2"}
	got := selectedPickerTarget(resolved, chosen, urlrequest.Request{}, picker.Selection{})
	if !got.Private || !got.NewWindow || got.OpenInBackground == nil || !*got.OpenInBackground {
		t.Fatalf("picker discarded required modes: %+v", got)
	}
	if got.Name != chosen.Name || got.ProfileDirectory != chosen.ProfileDirectory || len(got.Args) != 0 {
		t.Fatalf("picker replaced chosen browser identity with the resolved target: %+v", got)
	}
	if got.URL != resolved.URL || got.MatchedRule == nil || *got.MatchedRule != rule {
		t.Fatalf("picker lost link or rule provenance: %+v", got)
	}
}

func TestPickerShowsEffectiveModesAndAllowsExplicitBackgroundChange(t *testing.T) {
	for _, configuredBackground := range []bool{false, true} {
		resolved := browser.BrowserConfig{URL: "https://example.com", OpenInBackground: &configuredBackground}
		request := urlrequest.Request{Private: true, NewWindow: true}
		initial := requestForPicker(resolved, request, !configuredBackground, nil)
		if !initial.Private || !initial.NewWindow || initial.Background != configuredBackground {
			t.Fatalf("picker did not show effective launch modes: %+v", initial)
		}
		// An ordinary selection returns the initial controls unchanged.
		choice := picker.Selection{Background: initial.Background, BackgroundSpecified: true}
		got := selectedPickerTarget(resolved, browser.BrowserConfig{Name: "Google Chrome"}, request, choice)
		if !got.Private || !got.NewWindow || got.OpenInBackground == nil || *got.OpenInBackground != configuredBackground {
			t.Fatalf("ordinary selection changed requested modes: %+v", got)
		}
		choice.Background = !initial.Background
		got = selectedPickerTarget(resolved, browser.BrowserConfig{Name: "Google Chrome"}, request, choice)
		if *got.OpenInBackground != choice.Background {
			t.Fatal("explicit background control change was ignored")
		}
	}
}

func TestPickerPreservesFavoriteAndSelectedModes(t *testing.T) {
	chosen := browser.BrowserConfig{Name: "Google Chrome", Private: true, NewWindow: true}
	got := selectedPickerTarget(browser.BrowserConfig{}, chosen, urlrequest.Request{}, picker.Selection{})
	if !got.Private || !got.NewWindow {
		t.Fatal("favorite modes were discarded")
	}
	got = selectedPickerTarget(browser.BrowserConfig{}, browser.BrowserConfig{}, urlrequest.Request{}, picker.Selection{Private: true, NewWindow: true, Background: true})
	if !got.Private || !got.NewWindow || got.OpenInBackground == nil || !*got.OpenInBackground {
		t.Fatal("explicit picker modes were discarded")
	}
}

func TestAlternativeBrowserPreservesRequiredModes(t *testing.T) {
	background := true
	rule := 2
	for _, modeSource := range []string{"resolved", "request", "alternative"} {
		t.Run(modeSource, func(t *testing.T) {
			resolved := browser.BrowserConfig{
				Name: "Google Chrome", ProfileDirectory: "Profile 1", URL: "https://example.com/private",
				OpenInBackground: &background, MatchedRule: &rule, Args: []string{"--browser-specific-flag"},
			}
			request := urlrequest.Request{}
			features := &rules.Features{AlternativeBrowser: "com.microsoft.edgemac", AlternativeProfile: "Work", AlternativeProfileDirectory: "Profile 2"}
			switch modeSource {
			case "resolved":
				resolved.Private, resolved.NewWindow = true, true
			case "request":
				request.Private, request.NewWindow = true, true
			case "alternative":
				features.AlternativePrivate = true
			}
			got := alternativeBrowserTarget(resolved, request, features)
			if !got.Private || got.NewWindow != (modeSource != "alternative") || got.OpenInBackground == nil || !*got.OpenInBackground {
				t.Fatalf("alternative discarded required modes: %+v", got)
			}
			if got.Name != features.AlternativeBrowser || got.AppType != "bundleId" || got.Profile != "Work" || got.ProfileDirectory != "Profile 2" || len(got.Args) != 0 {
				t.Fatalf("alternative inherited the wrong browser identity or arguments: %+v", got)
			}
			if got.URL != resolved.URL || got.MatchedRule == nil || *got.MatchedRule != rule {
				t.Fatalf("alternative lost URL or rule provenance: %+v", got)
			}
		})
	}
}

func TestRemovingVisualOptionsRestoresJSOptions(t *testing.T) {
	target, err := config.NewFromScript(finickyConfigAPIJS, "finickyConfig", `var finickyConfig = {
		defaultBrowser: "Safari",
		options: { logRequests: false, hideIcon: false, checkForUpdates: true, keepRunning: true, transformScript: "return url;" }
	}`)
	if err != nil {
		t.Fatal(err)
	}
	target.SetIsJSConfig(true)
	on, off := true, false
	all := rules.RulesFile{Options: &rules.Options{LogRequests: &on, HideIcon: &on, CheckForUpdates: &off, KeepRunning: &off}}
	if err := applyVisualOptions(target, all); err != nil {
		t.Fatal(err)
	}
	got := target.GetAllConfigOptions()
	if !got.LogRequests || !got.HideIcon || got.CheckForUpdates || got.KeepRunning {
		t.Fatalf("overrides did not apply: %+v", got)
	}
	// Importing a partial options object removes the previous logging override.
	if err := applyVisualOptions(target, rules.RulesFile{Options: &rules.Options{HideIcon: &on}}); err != nil {
		t.Fatal(err)
	}
	got = target.GetAllConfigOptions()
	if got.LogRequests || !got.HideIcon || !got.CheckForUpdates || !got.KeepRunning {
		t.Fatalf("removed options remained overridden: %+v", got)
	}
	if err := applyVisualOptions(target, rules.RulesFile{}); err != nil {
		t.Fatal(err)
	}
	got = target.GetAllConfigOptions()
	if got.LogRequests || got.HideIcon || !got.CheckForUpdates || !got.KeepRunning {
		t.Fatalf("nil options did not restore the JS configuration: %+v", got)
	}
	transform, err := target.Runtime().RunString("finalConfig.options.transformScript")
	if err != nil || transform.String() != "return url;" {
		t.Fatalf("unrelated JS option was lost: %v %v", transform, err)
	}
}

func TestRemovingVisualOptionsRestoresImplicitDefaults(t *testing.T) {
	target, err := config.NewFromScript(finickyConfigAPIJS, "finickyConfig", `var finickyConfig = { defaultBrowser: "Safari" }`)
	if err != nil {
		t.Fatal(err)
	}
	on := true
	for _, rf := range []rules.RulesFile{{Options: &rules.Options{LogRequests: &on}}, {}} {
		if err := applyVisualOptions(target, rf); err != nil {
			t.Fatal(err)
		}
	}
	if target.GetAllConfigOptions().LogRequests {
		t.Fatal("missing JS logging option retained a removed true override")
	}
}

func TestRejectedFocusActivationDoesNotPoisonLaterRestoration(t *testing.T) {
	rules.SetCustomPath(filepath.Join(t.TempDir(), "rules.json"))
	t.Cleanup(func() { rules.SetCustomPath("") })
	rf := rules.RulesFile{DefaultBrowser: "Google Chrome", DefaultProfileDirectory: "Default"}
	path, err := focusDefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	for _, request := range []urlrequest.Request{
		{Focus: "active", Browser: "Safari", ProfileDirectory: "Default"},
		{Focus: "active", Browser: "Safari", Private: true},
		{Focus: "active", Browser: "Finicky Plus"},
	} {
		if err := applyAndSaveDefaultRequest(rf, request, rules.Save); err == nil {
			t.Fatalf("invalid target was saved: %+v", request)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("rejected activation left a snapshot: %v", err)
		}
	}
	// The user's next default is the one a later valid Focus must restore.
	rf.DefaultProfileDirectory = "Profile 1"
	if err := rules.Save(rf); err != nil {
		t.Fatal(err)
	}
	if err := applyAndSaveDefaultRequest(rf, urlrequest.Request{Focus: "active", Browser: "Google Chrome", ProfileDirectory: "Profile 2"}, rules.Save); err != nil {
		t.Fatal(err)
	}
	rf, err = rules.Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := applyAndSaveDefaultRequest(rf, urlrequest.Request{Focus: "clear"}, rules.Save); err != nil {
		t.Fatal(err)
	}
	restored, err := rules.Load()
	if err != nil || restored.DefaultProfileDirectory != "Profile 1" {
		t.Fatalf("wrong default restored: %+v %v", restored, err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("restored Focus retained its snapshot: %v", err)
	}
}

func TestFocusSaveFailureRemovesOnlyNewSnapshot(t *testing.T) {
	rules.SetCustomPath(filepath.Join(t.TempDir(), "rules.json"))
	t.Cleanup(func() { rules.SetCustomPath("") })
	rf := rules.RulesFile{DefaultBrowser: "Google Chrome", DefaultProfileDirectory: "Default"}
	request := urlrequest.Request{Focus: "active", Browser: "Google Chrome", ProfileDirectory: "Profile 1"}
	wantErr := errors.New("write failed")
	failSave := func(rules.RulesFile) error { return wantErr }
	if err := applyAndSaveDefaultRequest(rf, request, failSave); !errors.Is(err, wantErr) {
		t.Fatalf("save failure lost: %v", err)
	}
	path, _ := focusDefaultPath()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("failed initial activation left a snapshot: %v", err)
	}
	if err := applyAndSaveDefaultRequest(rf, request, rules.Save); err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	rf.DefaultProfileDirectory = "Profile 1"
	request.ProfileDirectory = "Profile 2"
	if err := applyAndSaveDefaultRequest(rf, request, failSave); !errors.Is(err, wantErr) {
		t.Fatalf("repeated activation failure lost: %v", err)
	}
	preserved, err := os.ReadFile(path)
	if err != nil || string(preserved) != string(original) {
		t.Fatalf("failed repeated activation changed the valid snapshot: %s %v", preserved, err)
	}
}

func TestFocusApplyFailureAfterSaveKeepsRestorationSnapshot(t *testing.T) {
	rules.SetCustomPath(filepath.Join(t.TempDir(), "rules.json"))
	t.Cleanup(func() { rules.SetCustomPath("") })
	rf := rules.RulesFile{DefaultBrowser: "Google Chrome", DefaultProfileDirectory: "Default"}
	wantErr := errors.New("could not apply runtime option")
	saveThenFail := func(proposed rules.RulesFile) error {
		if err := rules.Save(proposed); err != nil {
			return err
		}
		return &savedRulesApplyError{wantErr}
	}
	err := applyAndSaveDefaultRequest(rf, urlrequest.Request{Focus: "active", Browser: "Google Chrome", ProfileDirectory: "Profile 1"}, saveThenFail)
	if !errors.Is(err, wantErr) {
		t.Fatalf("apply error lost: %v", err)
	}
	rf, err = rules.Load()
	if err != nil || rf.DefaultProfileDirectory != "Profile 1" {
		t.Fatalf("test did not persist active default: %+v %v", rf, err)
	}
	if err := applyAndSaveDefaultRequest(rf, urlrequest.Request{Focus: "clear"}, rules.Save); err != nil {
		t.Fatal(err)
	}
	rf, err = rules.Load()
	if err != nil || rf.DefaultProfileDirectory != "Default" {
		t.Fatalf("persisted Focus lost restoration after apply failure: %+v %v", rf, err)
	}
}

func TestPersistedFocusClearConsumesSnapshotDespiteApplyError(t *testing.T) {
	rules.SetCustomPath(filepath.Join(t.TempDir(), "rules.json"))
	t.Cleanup(func() { rules.SetCustomPath("") })
	rf := rules.RulesFile{DefaultBrowser: "Google Chrome", DefaultProfileDirectory: "Default"}
	activate := urlrequest.Request{Focus: "active", Browser: "Google Chrome", ProfileDirectory: "Profile 1"}
	if err := applyAndSaveDefaultRequest(rf, activate, rules.Save); err != nil {
		t.Fatal(err)
	}
	rf, err := rules.Load()
	if err != nil {
		t.Fatal(err)
	}
	wantErr := errors.New("could not apply runtime option")
	saveThenFail := func(proposed rules.RulesFile) error {
		if err := rules.Save(proposed); err != nil {
			return err
		}
		return &savedRulesApplyError{wantErr}
	}
	if err := applyAndSaveDefaultRequest(rf, urlrequest.Request{Focus: "clear"}, saveThenFail); !errors.Is(err, wantErr) {
		t.Fatalf("runtime error was not preserved: %v", err)
	}
	path, _ := focusDefaultPath()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("persisted restore left a stale snapshot: %v", err)
	}
	rf, err = rules.Load()
	if err != nil || rf.DefaultProfileDirectory != "Default" {
		t.Fatalf("restore was not persisted: %+v %v", rf, err)
	}
	// A later manual default must be captured by the next Focus session.
	rf.DefaultProfileDirectory = "Profile 2"
	if err := rules.Save(rf); err != nil {
		t.Fatal(err)
	}
	if err := applyAndSaveDefaultRequest(rf, activate, rules.Save); err != nil {
		t.Fatal(err)
	}
	rf, err = rules.Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := applyAndSaveDefaultRequest(rf, urlrequest.Request{Focus: "clear"}, rules.Save); err != nil {
		t.Fatal(err)
	}
	rf, err = rules.Load()
	if err != nil || rf.DefaultProfileDirectory != "Profile 2" {
		t.Fatalf("next Focus restored an older default: %+v %v", rf, err)
	}
}

func TestFailedFocusClearKeepsSnapshotUntilRestoreIsSaved(t *testing.T) {
	rules.SetCustomPath(filepath.Join(t.TempDir(), "rules.json"))
	t.Cleanup(func() { rules.SetCustomPath("") })
	rf := rules.RulesFile{DefaultBrowser: "Google Chrome", DefaultProfileDirectory: "Default"}
	if err := applyAndSaveDefaultRequest(rf, urlrequest.Request{Focus: "active", Browser: "Google Chrome", ProfileDirectory: "Profile 1"}, rules.Save); err != nil {
		t.Fatal(err)
	}
	path, _ := focusDefaultPath()
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	rf, err = rules.Load()
	if err != nil {
		t.Fatal(err)
	}
	wantErr := errors.New("write failed")
	if err := applyAndSaveDefaultRequest(rf, urlrequest.Request{Focus: "clear"}, func(rules.RulesFile) error { return wantErr }); !errors.Is(err, wantErr) {
		t.Fatalf("save error was not preserved: %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(before) {
		t.Fatalf("failed restore lost its snapshot: %s %v", after, err)
	}
	if err := applyAndSaveDefaultRequest(rf, urlrequest.Request{Focus: "clear"}, rules.Save); err != nil {
		t.Fatal(err)
	}
	rf, err = rules.Load()
	if err != nil || rf.DefaultProfileDirectory != "Default" {
		t.Fatalf("retry did not restore the original default: %+v %v", rf, err)
	}
}
