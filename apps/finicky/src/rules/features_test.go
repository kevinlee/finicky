package rules

import (
	"encoding/json"
	"testing"
)

func TestFeaturesAndFavoriteFieldsRoundTrip(t *testing.T) {
	no := false
	rf := RulesFile{Features: &Features{PromptMode: "unmatched", PromptModifier: "fn", ExpandShortURLs: &no, HistoryEnabled: true, ExtraShorteners: []string{"short.example"}}, Profiles: []SavedProfile{{ID: "work", Browser: "Google Chrome", Private: true, Hidden: true, Shortcut: "w"}}}
	encoded, err := json.Marshal(rf)
	if err != nil {
		t.Fatal(err)
	}
	var decoded RulesFile
	if err = json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Features == nil || decoded.Features.ExpandShortURLs == nil || *decoded.Features.ExpandShortURLs || !decoded.Profiles[0].Private || !decoded.Profiles[0].Hidden || decoded.Profiles[0].Shortcut != "w" {
		t.Fatal("lost feature/favorite fields")
	}
}

func TestInvalidFeaturesRejected(t *testing.T) {
	for _, f := range []*Features{{PromptMode: "sometimes"}, {PromptModifier: "alt"}, {ExtraShorteners: []string{"https://example.com"}}, {AlternativeProfile: "Work"}, {AlternativeBrowser: "Safari", AlternativeProfileDirectory: "Default"}} {
		if ValidateFeatures(f) == nil {
			t.Fatalf("accepted invalid features %#v", f)
		}
	}
}

func TestRulesCarryPrivateAndNewWindow(t *testing.T) {
	handlers := ToJSHandlers([]Rule{{Browser: "Google Chrome", Match: []string{"*"}, Private: true, NewWindow: true}})
	target := handlers[0]["browser"].(map[string]interface{})
	if target["private"] != true || target["newWindow"] != true {
		t.Fatal("private/new window lost during JS conversion")
	}
}

func TestNativeServiceSelectionRetainsEmptyVersusDefault(t *testing.T) {
	for _, services := range [][]string{nil, {}, {"zoom", "spotify"}} {
		data, err := json.Marshal(Features{NativeAppServices: services})
		if err != nil {
			t.Fatal(err)
		}
		var restored Features
		if err := json.Unmarshal(data, &restored); err != nil {
			t.Fatal(err)
		}
		if (services == nil) != (restored.NativeAppServices == nil) || len(services) != len(restored.NativeAppServices) {
			t.Fatalf("native selection changed: %s", data)
		}
	}
}
