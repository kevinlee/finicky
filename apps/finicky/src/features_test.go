package main

import (
	"finicky/browser"
	"finicky/rules"
	"finicky/urlrequest"
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
