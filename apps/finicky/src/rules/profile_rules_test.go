package rules_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	. "finicky/rules"
)

func TestProfileRulesRoundTripAndBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rules.json")
	disabled := false
	first := RulesFile{DefaultBrowser: "Safari", Rules: []Rule{}}
	if err := SaveToPath(first, path); err != nil {
		t.Fatal(err)
	}
	firstBytes, _ := os.ReadFile(path)
	wanted := RulesFile{
		DefaultBrowser: "Google Chrome", DefaultProfile: "Personal", DefaultProfileDirectory: "Default",
		Profiles: []SavedProfile{{ID: "work", Name: "Work", Browser: "Google Chrome", ProfileDirectory: "Profile 2", Color: "blue"}},
		Options:  &Options{KeepRunning: &disabled},
		Rules:    []Rule{{Name: "Work apps", SourceApps: []string{"com.tinyspeck.slackmacgap"}, Browser: "Google Chrome", Profile: "Work", ProfileDirectory: "Profile 2", Enabled: &disabled}},
	}
	if err := SaveToPath(wanted, path); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadFromPath(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(wanted, loaded) {
		t.Fatalf("round trip changed fields: %#v", loaded)
	}
	backup, err := os.ReadFile(path + ".backup")
	if err != nil || string(backup) != string(firstBytes) {
		t.Fatalf("previous version was not preserved: %v", err)
	}
	wanted.DefaultProfileDirectory = "../bad"
	if err := SaveToPath(wanted, path); err == nil {
		t.Fatal("unsafe directory accepted")
	}
	after, _ := LoadFromPath(path)
	if !reflect.DeepEqual(loaded, after) {
		t.Fatal("invalid save changed existing rules")
	}
}

func TestRuleLegacyStringAndNewFields(t *testing.T) {
	var rule Rule
	data := []byte(`{"match":"example.com/*","browser":"Google Chrome","sourceApps":["com.app"],"name":"Example","profileDirectory":"Profile 2","enabled":false}`)
	if err := json.Unmarshal(data, &rule); err != nil {
		t.Fatal(err)
	}
	if len(rule.Match) != 1 || rule.Name != "Example" || rule.Enabled == nil || *rule.Enabled {
		t.Fatalf("incorrect rule: %#v", rule)
	}
	encoded, err := json.Marshal(rule)
	if err != nil {
		t.Fatal(err)
	}
	var decoded Rule
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(rule, decoded) {
		t.Fatal("rule fields did not round trip")
	}
}

func TestAppOnlyAndDisabledHandlers(t *testing.T) {
	disabled := false
	handlers := ToJSHandlers([]Rule{
		{SourceApps: []string{"com.app"}, Browser: "Google Chrome", ProfileDirectory: "Profile 2"},
		{Match: []string{"*"}, Browser: "Firefox", Enabled: &disabled},
		{SourceApps: []string{" "}, Browser: "Safari"},
	})
	if len(handlers) != 1 {
		t.Fatalf("got %d handlers", len(handlers))
	}
	if handlers[0]["match"] != "*" {
		t.Fatal("app-only rule does not match all URLs")
	}
	if !reflect.DeepEqual(handlers[0]["sourceApps"], []string{"com.app"}) {
		t.Fatal("lost source app")
	}
	target := handlers[0]["browser"].(map[string]interface{})
	if target["profileDirectory"] != "Profile 2" {
		t.Fatal("lost explicit directory")
	}
}

func TestValidateRejectsUnsupportedProfileDirectoryAndSourceApp(t *testing.T) {
	for _, rf := range []RulesFile{
		{DefaultBrowser: "Safari", DefaultProfileDirectory: "Default"},
		{Rules: []Rule{{Browser: "Firefox", ProfileDirectory: "Profile 2"}}},
		{Profiles: []SavedProfile{{Name: "Unknown", Browser: "Unknown", ProfileDirectory: "Default"}}},
		{Rules: []Rule{{Browser: "Safari", SourceApps: []string{"/Applications/Slack.app"}}}},
	} {
		if err := Validate(rf); err == nil {
			t.Errorf("accepted invalid settings: %#v", rf)
		}
	}
}
