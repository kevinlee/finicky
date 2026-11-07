package browser

import (
	"reflect"
	"testing"
)

func TestDirectProfileArgs(t *testing.T) {
	for _, identifier := range []string{"Google Chrome", "com.google.Chrome"} {
		args, err := directProfileArgs(identifier, "Profile 2")
		if err != nil || !reflect.DeepEqual(args, []string{"--profile-directory=Profile 2"}) {
			t.Fatalf("%s: args = %v, error = %v", identifier, args, err)
		}
	}
}

func TestDirectProfileArgsRejectsInvalidTarget(t *testing.T) {
	for _, directory := range []string{"", " ", ".", "..", "../Profile 2", "a/b", "a\\b", "a\x00b", "a\nb"} {
		if _, err := directProfileArgs("Google Chrome", directory); err == nil {
			t.Errorf("accepted invalid directory %q", directory)
		}
	}
	for _, browser := range []string{"Firefox", "Safari", "Unknown Browser"} {
		if _, err := directProfileArgs(browser, "Default"); err == nil {
			t.Errorf("accepted unsupported browser %q", browser)
		}
	}
}

func TestBuildOpenArgsKeepsProfileAfterSeparator(t *testing.T) {
	cfg := BrowserConfig{Name: "com.google.Chrome", AppType: "bundleId", ProfileDirectory: "Profile 2", URL: "https://example.com"}
	args, err := buildOpenArgs(cfg, false)
	want := []string{"-b", "com.google.Chrome", "-n", "--args", "--profile-directory=Profile 2", "https://example.com"}
	if err != nil || !reflect.DeepEqual(args, want) {
		t.Fatalf("args=%v error=%v", args, err)
	}
	cfg.Args = []string{"-n", "--args", "--incognito", cfg.URL}
	args, err = buildOpenArgs(cfg, false)
	want = []string{"-b", "com.google.Chrome", "-n", "--args", "--profile-directory=Profile 2", "--incognito", cfg.URL}
	if err != nil || !reflect.DeepEqual(args, want) {
		t.Fatalf("custom args=%v error=%v", args, err)
	}
}

func TestBuildOpenArgsPreservesLegacyCustomDirectoryWorkaround(t *testing.T) {
	cfg := BrowserConfig{Name: "Google Chrome", AppType: "appName", Args: []string{"-n", "--args", "--profile-directory=Profile 2", "https://example.com"}}
	args, err := buildOpenArgs(cfg, false)
	want := []string{"-a", "Google Chrome", "-n", "--args", "--profile-directory=Profile 2", "https://example.com"}
	if err != nil || !reflect.DeepEqual(args, want) {
		t.Fatalf("args=%v error=%v", args, err)
	}
}
