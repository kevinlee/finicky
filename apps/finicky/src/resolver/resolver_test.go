package resolver_test

import (
	"os"
	"testing"

	"finicky/config"
	. "finicky/resolver"
	"finicky/rules"
)

// apiContent reads finickyConfigAPI.js relative to this package directory.
// go test sets the working directory to the package source directory, so
// "../assets/..." resolves correctly.
func apiContent(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("../assets/finickyConfigAPI.js")
	if err != nil {
		t.Fatalf("failed to load finickyConfigAPI.js: %v\n(run from apps/finicky/src/resolver/)", err)
	}
	return b
}

// jsVM creates a VM from an inline JS config object literal.
// NewFromScript bypasses esbuild bundling, so we use the legacy var-assignment
// syntax that the config API accepts directly in goja.
// The VM is marked as a JS-config VM to exercise the merge path.
func jsVM(t *testing.T, configObj string) *config.VM {
	t.Helper()
	script := "var finickyConfig = " + configObj
	vm, err := config.NewFromScript(apiContent(t), "finickyConfig", script)
	if err != nil {
		t.Fatalf("failed to create VM from JS: %v", err)
	}
	vm.SetIsJSConfig(true)
	return vm
}

// rulesVM creates a VM from a RulesFile (the no-JS-config path).
func rulesVM(t *testing.T, rf rules.RulesFile) *config.VM {
	t.Helper()
	script, err := rules.ToJSConfigScript(rf, "finickyConfig")
	if err != nil {
		t.Fatalf("failed to generate JS config from rules: %v", err)
	}
	vm, err := config.NewFromScript(apiContent(t), "finickyConfig", script)
	if err != nil {
		t.Fatalf("failed to create VM from rules: %v", err)
	}
	return vm
}

func TestResolveURL_NoConfig(t *testing.T) {
	result, err := ResolveURL(nil, "https://example.com", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Name != "com.apple.Safari" {
		t.Errorf("got %q, want %q", result.Name, "com.apple.Safari")
	}
}

func TestResolveURL_JSConfig(t *testing.T) {
	vm := jsVM(t, `({
		defaultBrowser: "Safari",
		handlers: [
			{ match: "*github.com/*", browser: "Firefox" },
			{ match: "https://linear.app/*", browser: "Google Chrome" }
		]
	})`)

	tests := []struct {
		url     string
		browser string
	}{
		{"https://github.com/johnste/finicky", "Firefox"},
		{"https://gist.github.com/foo", "Firefox"},
		{"https://linear.app/team/issue/123", "Google Chrome"},
		{"https://example.com", "Safari"},
	}
	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			result, err := ResolveURL(vm, tt.url, nil, false)
			if err != nil {
				t.Fatal(err)
			}
			if result.Name != tt.browser {
				t.Errorf("got %q, want %q", result.Name, tt.browser)
			}
		})
	}
}

func TestResolveURL_JSONRulesOnly(t *testing.T) {
	rf := rules.RulesFile{
		DefaultBrowser: "Firefox",
		Rules: []rules.Rule{
			{Match: []string{"*github.com/*"}, Browser: "Google Chrome"},
			{Match: []string{"https://linear.app/*"}, Browser: "Safari"},
		},
	}
	vm := rulesVM(t, rf)

	tests := []struct {
		url     string
		browser string
	}{
		{"https://github.com/johnste/finicky", "Google Chrome"},
		{"https://linear.app/team/issue/123", "Safari"},
		{"https://example.com", "Firefox"},
	}
	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			result, err := ResolveURL(vm, tt.url, nil, false)
			if err != nil {
				t.Fatal(err)
			}
			if result.Name != tt.browser {
				t.Errorf("got %q, want %q", result.Name, tt.browser)
			}
		})
	}
}

func TestResolveURL_JSONRulesWithProfile(t *testing.T) {
	rf := rules.RulesFile{
		DefaultBrowser: "Safari",
		Rules: []rules.Rule{
			{Match: []string{"*github.com/*"}, Browser: "Google Chrome", Profile: "Work"},
		},
	}
	vm := rulesVM(t, rf)

	result, err := ResolveURL(vm, "https://github.com/foo", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Name != "Google Chrome" {
		t.Errorf("browser: got %q, want %q", result.Name, "Google Chrome")
	}
	if result.Profile != "Work" {
		t.Errorf("profile: got %q, want %q", result.Profile, "Work")
	}
}

// TestResolveURL_MergedJSAndJSON verifies that JS handlers take precedence
// over JSON rules handlers when both are present.
func TestResolveURL_MergedJSAndJSON(t *testing.T) {
	// JS config handles github. jsVM sets IsJSConfig=true so the merge path
	// is exercised. With no rules cached, _jsonHandlers is [], so finalConfig
	// is used as-is — JS handlers apply normally.
	jsConfig := jsVM(t, `({
		defaultBrowser: "Safari",
		handlers: [
			{ match: "*github.com/*", browser: "Firefox" }
		]
	})`)

	tests := []struct {
		url     string
		browser string
	}{
		{"https://github.com/foo", "Firefox"}, // matched by JS handler
		{"https://example.com", "Safari"},     // falls through to JS default
	}
	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			result, err := ResolveURL(jsConfig, tt.url, nil, false)
			if err != nil {
				t.Fatal(err)
			}
			if result.Name != tt.browser {
				t.Errorf("got %q, want %q", result.Name, tt.browser)
			}
		})
	}
}

// TestResolveURL_MergedJSAndJSON_WithCachedRules verifies that JSON rules are
// applied when a JS config is present and cached rules have been seeded
// (simulating the fix for the startup bug where SetCachedRules was not called
// when a JS config existed).
func TestResolveURL_MergedJSAndJSON_WithCachedRules(t *testing.T) {
	jsConfig := jsVM(t, `({
		defaultBrowser: "Safari",
		handlers: [
			{ match: "*github.com/*", browser: "Firefox" }
		]
	})`)

	SetCachedRules(rules.RulesFile{
		DefaultBrowser: "Safari",
		Rules: []rules.Rule{
			{Match: []string{"linear.app/*"}, Browser: "Google Chrome"},
		},
	})
	t.Cleanup(func() { SetCachedRules(rules.RulesFile{}) })

	tests := []struct {
		url     string
		browser string
	}{
		{"https://github.com/foo", "Firefox"},                // JS handler wins
		{"https://linear.app/team/issue/1", "Google Chrome"}, // JSON rule applies
		{"https://example.com", "Safari"},                    // JS default
	}
	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			result, err := ResolveURL(jsConfig, tt.url, nil, false)
			if err != nil {
				t.Fatal(err)
			}
			if result.Name != tt.browser {
				t.Errorf("got %q, want %q", result.Name, tt.browser)
			}
		})
	}
}

func TestResolveURL_JSConfigFunctionHandler(t *testing.T) {
	vm := jsVM(t, `({
		defaultBrowser: "Safari",
		handlers: [{
			match: function(request, { opener }) {
				return opener && opener.bundleId === "com.apple.Terminal";
			},
			browser: "Firefox"
		}]
	})`)

	terminal := &OpenerInfo{Name: "Terminal", BundleID: "com.apple.Terminal", Path: "/Applications/Utilities/Terminal.app"}
	other := &OpenerInfo{Name: "Finder", BundleID: "com.apple.finder", Path: "/System/Library/CoreServices/Finder.app"}

	result, err := ResolveURL(vm, "https://example.com", terminal, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Name != "Firefox" {
		t.Errorf("terminal opener: got %q, want %q", result.Name, "Firefox")
	}

	result, err = ResolveURL(vm, "https://example.com", other, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Name != "Safari" {
		t.Errorf("other opener: got %q, want %q", result.Name, "Safari")
	}
}

func TestResolveURL_RewriteRule(t *testing.T) {
	vm := jsVM(t, `({
		defaultBrowser: "Safari",
		rewrite: [{
			match: "https://www.youtube.com/watch*",
			url: function(url) {
				return url.href.replace("https://www.youtube.com/watch", "https://youtu.be/");
			}
		}]
	})`)

	result, err := ResolveURL(vm, "https://www.youtube.com/watch?v=dQw4w9WgXcQ", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.URL != "https://youtu.be/?v=dQw4w9WgXcQ" {
		t.Errorf("rewritten URL: got %q", result.URL)
	}
}

func TestResolveURL_OpenInBackground(t *testing.T) {
	result, err := ResolveURL(nil, "https://example.com", nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if result.OpenInBackground == nil || !*result.OpenInBackground {
		t.Error("expected OpenInBackground=true")
	}
}

func TestResolveURL_AppRulesAndExplicitDirectories(t *testing.T) {
	disabled := false
	rf := rules.RulesFile{
		DefaultBrowser: "Google Chrome", DefaultProfileDirectory: "Default",
		Rules: []rules.Rule{
			{SourceApps: []string{"com.slack"}, Browser: "Firefox", Enabled: &disabled},
			{SourceApps: []string{"com.slack"}, Match: []string{"work.example/*"}, Browser: "Google Chrome", Profile: "Work", ProfileDirectory: "Profile 2"},
			{SourceApps: []string{"com.cursor"}, Browser: "Google Chrome", ProfileDirectory: "Profile 1"},
		},
	}
	vm := rulesVM(t, rf)
	for _, test := range []struct{ url, source, directory string }{
		{"https://work.example/doc", "com.slack", "Profile 2"},
		{"https://elsewhere.example", "com.slack", "Default"},
		{"https://anywhere.example", "com.cursor", "Profile 1"},
		{"https://work.example/doc", "", "Default"},
	} {
		var opener *OpenerInfo
		if test.source != "" {
			opener = &OpenerInfo{BundleID: test.source, Name: test.source}
		}
		result, err := ResolveURL(vm, test.url, opener, false)
		if err != nil {
			t.Fatal(err)
		}
		if result.ProfileDirectory != test.directory {
			t.Errorf("%s from %s: got %q, want %q", test.url, test.source, result.ProfileDirectory, test.directory)
		}
	}
}

func TestResolveURL_AppRulesPreserveJSPrecedence(t *testing.T) {
	vm := jsVM(t, `({defaultBrowser: "Safari", handlers: [{match: "work.example/*", browser: "Firefox"}]})`)
	SetCachedRules(rules.RulesFile{Rules: []rules.Rule{
		{SourceApps: []string{"com.slack"}, Browser: "Google Chrome", ProfileDirectory: "Profile 2"},
	}})
	t.Cleanup(func() { SetCachedRules(rules.RulesFile{}) })
	opener := &OpenerInfo{Name: "Slack", BundleID: "com.slack"}
	for _, test := range []struct{ url, browser, directory string }{
		{"https://work.example/doc", "Firefox", ""},
		{"https://other.example/doc", "Google Chrome", "Profile 2"},
	} {
		result, err := ResolveURL(vm, test.url, opener, false)
		if err != nil {
			t.Fatal(err)
		}
		if result.Name != test.browser || result.ProfileDirectory != test.directory {
			t.Errorf("%s: got %+v", test.url, result)
		}
	}
}

func TestSavedDefaultOverridesOnlyJSFallback(t *testing.T) {
	vm := jsVM(t, `({defaultBrowser: "Safari", handlers: [{match: "work.example/*", browser: "Firefox"}]})`)
	SetCachedRules(rules.RulesFile{DefaultBrowser: "Google Chrome", DefaultProfileDirectory: "Profile 2"})
	t.Cleanup(func() { SetCachedRules(rules.RulesFile{}) })
	matched, err := ResolveURL(vm, "https://work.example/page", nil, false)
	if err != nil || matched.Name != "Firefox" || matched.MatchedRule == nil {
		t.Fatalf("matched: %#v %v", matched, err)
	}
	fallback, err := ResolveURL(vm, "https://personal.example/page", nil, false)
	if err != nil || fallback.Name != "Google Chrome" || fallback.ProfileDirectory != "Profile 2" {
		t.Fatalf("fallback: %#v %v", fallback, err)
	}
}
func TestCleanupWithoutJSConfig(t *testing.T) {
	off := false
	SetCachedRules(rules.RulesFile{Features: &rules.Features{StripTracking: true, ExpandShortURLs: &off}})
	t.Cleanup(func() { SetCachedRules(rules.RulesFile{}) })
	result, err := ResolveURL(nil, "https://example.com/path?utm_source=test&file=keep", nil, false)
	if err != nil || result.URL != "https://example.com/path?file=keep" {
		t.Fatalf("%#v %v", result, err)
	}
}

func TestVisualTransformsAndStableRuleIndex(t *testing.T) {
	for _, js := range []bool{false, true} {
		off := false
		rf := rules.RulesFile{DefaultBrowser: "Safari", Features: &rules.Features{TransformScript: `return url.replace("old.example", "new.example");`}, Rules: []rules.Rule{
			{Match: []string{"*"}, Browser: "Safari", Enabled: &off},
			{Match: []string{"https://new.example/*"}, Browser: "Firefox", TransformScript: `return url.replace("/start", "/finish");`},
		}}
		SetCachedRules(rf)
		vm := rulesVM(t, rf)
		if js {
			vm = jsVM(t, `({defaultBrowser: "Safari"})`)
		}
		got, err := ResolveURL(vm, "https://old.example/start", nil, false)
		if err != nil || got.URL != "https://new.example/finish" || got.Name != "Firefox" || got.MatchedRule == nil || *got.MatchedRule != 1 {
			t.Fatalf("js=%v: %+v %v", js, got, err)
		}
	}
	SetCachedRules(rules.RulesFile{})
}

func TestTransformTimeoutDoesNotPoisonLaterRequests(t *testing.T) {
	vm := jsVM(t, `({defaultBrowser: "Safari", handlers: [{match: function(url) { if(url.hostname === "loop.example") { while(true) {} } return false; }, browser: "Firefox"}]})`)
	if _, err := ResolveURL(vm, "https://loop.example", nil, false); err == nil {
		t.Fatal("loop was not interrupted")
	}
	if result, err := ResolveURL(vm, "https://example.com", nil, false); err != nil || result.Name != "Safari" {
		t.Fatalf("later request failed: %+v %v", result, err)
	}
}
