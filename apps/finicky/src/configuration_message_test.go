package main

import (
	"finicky/config"
	"finicky/rules"
	"reflect"
	"testing"
)

func TestConfigurationMessageRefreshesRemovedOptionOverrides(t *testing.T) {
	target, err := config.NewFromScript(finickyConfigAPIJS, "finickyConfig", `var finickyConfig = {
		defaultBrowser: "Firefox",
		handlers: [{match: "work.example/*", browser: "Google Chrome"}],
		rewrite: [{match: "old.example/*", url: "https://new.example"}],
		options: {logRequests: false, keepRunning: true}
	}`)
	if err != nil {
		t.Fatal(err)
	}
	target.SetIsJSConfig(true)
	on := true
	if err := applyVisualOptions(target, rules.RulesFile{Options: &rules.Options{LogRequests: &on}}); err != nil {
		t.Fatal(err)
	}
	path := "/tmp/finicky-qa/config.js"
	beforeInfo, before := configurationSnapshot(target, path)
	if !before["options"].(map[string]interface{})["logRequests"].(bool) {
		t.Fatal("initial message omitted the saved override")
	}
	if err := applyVisualOptions(target, rules.RulesFile{}); err != nil {
		t.Fatal(err)
	}
	afterInfo, after := configurationSnapshot(target, path)
	if after["options"].(map[string]interface{})["logRequests"].(bool) {
		t.Fatal("refreshed message still exposes the removed override")
	}
	if !reflect.DeepEqual(beforeInfo, afterInfo) || afterInfo.ConfigPath != path || afterInfo.Handlers != 1 || afterInfo.Rewrites != 1 || afterInfo.DefaultBrowser != "Firefox" {
		t.Fatalf("refresh lost routing configuration: before=%+v after=%+v", beforeInfo, afterInfo)
	}
	for _, key := range []string{"handlers", "rewrites", "defaultBrowser", "configPath", "isJSConfig"} {
		if !reflect.DeepEqual(before[key], after[key]) {
			t.Errorf("refresh changed %s: %v -> %v", key, before[key], after[key])
		}
	}
}

func TestEmptyConfigurationMessageClearsPreviousState(t *testing.T) {
	info, message := configurationSnapshot(nil, "")
	if info.Handlers != 0 || info.Rewrites != 0 || info.ConfigPath != "" || info.DefaultBrowser != "Safari" {
		t.Fatalf("empty configuration retained previous routing state: %+v", info)
	}
	if message["isJSConfig"] != false || message["configPath"] != "" {
		t.Fatalf("empty configuration retained a source: %+v", message)
	}
	want := map[string]interface{}{
		"keepRunning": true, "hideIcon": false, "logRequests": false, "checkForUpdates": true,
	}
	if !reflect.DeepEqual(message["options"], want) {
		t.Fatalf("empty configuration did not publish default options: got %v want %v", message["options"], want)
	}
}
