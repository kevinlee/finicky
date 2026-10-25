package browser

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestPrivateAndWindowLaunchArgs(t *testing.T) {
	for _, test := range []struct {
		name               string
		private, newWindow bool
		want               []string
	}{
		{"Google Chrome", true, false, []string{"--incognito"}},
		{"Microsoft Edge", true, true, []string{"--inprivate", "--new-window"}},
		{"Firefox", true, true, []string{"-private-window"}},
		{"Zen", false, true, []string{"-new-window"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg := BrowserConfig{Name: test.name, Private: test.private, NewWindow: test.newWindow, URL: "https://example.com/?a=1&b=2"}
			args, err := buildOpenArgs(cfg, false)
			want := append([]string{"-a", test.name, "-n", "--args"}, test.want...)
			want = append(want, cfg.URL)
			if err != nil || !reflect.DeepEqual(args, want) {
				t.Fatalf("args=%v err=%v want=%v", args, err, want)
			}
		})
	}
}

func TestUnsafeOrUnresolvedLaunchIsRejected(t *testing.T) {
	for _, cfg := range []BrowserConfig{
		{Name: "Safari", Private: true},
		{Name: "Unknown", NewWindow: true},
		{Name: "Unknown", Profile: "Work"},
		{Name: "Google Chrome", Profile: "finicky-profile-does-not-exist-815523"},
		{Name: "com.kevinlee.finicky"},
		{Name: "/Applications/Finicky Plus.app"},
		{Name: "Waystile"},
		{Name: "/Applications/Waystile.app"},
		{Name: "/Applications/Waystile Preview.app"},
		{Name: "se.johnste.finicky"},
	} {
		if _, err := buildOpenArgs(cfg, false); err == nil {
			t.Fatalf("accepted %#v", cfg)
		}
	}
	if err := LaunchBrowser(BrowserConfig{Action: "prompt", AppType: "none"}, true, false); err == nil {
		t.Fatal("unresolved prompt treated as successful launch")
	}
}

func TestProfilePrivateAndLegacyCustomArgs(t *testing.T) {
	cfg := BrowserConfig{Name: "Google Chrome", ProfileDirectory: "Profile 2", Private: true, URL: "https://example.com", Args: []string{"-n", "--args", "--new-window", "https://example.com"}}
	args, err := buildOpenArgs(cfg, false)
	want := []string{"-a", "Google Chrome", "-n", "--args", "--profile-directory=Profile 2", "--incognito", "--new-window", "https://example.com"}
	if err != nil || !reflect.DeepEqual(args, want) {
		t.Fatalf("args=%v err=%v", args, err)
	}
}

func TestProfilesKeepStableDirectoryIDsAndRejectAmbiguousNames(t *testing.T) {
	dir := t.TempDir()
	state := filepath.Join(dir, "Local State")
	if err := os.WriteFile(state, []byte(`{"profile":{"info_cache":{"Profile 2":{"name":"Work"},"Default":{"name":"Work"}}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, ok := parseProfiles(state, "Work"); ok {
		t.Fatal("ambiguous display name picked a random account")
	}
	if got, ok := parseProfiles(state, "Profile 2"); !ok || got != "Profile 2" {
		t.Fatal("stable directory did not resolve")
	}
	profiles := readBrowserProfiles(*lookupBrowser("Google Chrome"), dir)
	if len(profiles) != 2 || profiles[0].ID == profiles[1].ID {
		t.Fatalf("profile IDs not distinct: %#v", profiles)
	}
	for _, p := range profiles {
		if !strings.HasSuffix(p.ID, p.ProfileDirectory) {
			t.Fatalf("ID does not include stable directory: %#v", p)
		}
	}
}

func TestCanonicalBrowserIdentifiers(t *testing.T) {
	for _, identifier := range []string{"Google Chrome", "com.google.Chrome", "/Applications/Google Chrome.app"} {
		if got := CanonicalIdentifier(identifier); got != "com.google.Chrome" {
			t.Errorf("canonical(%q)=%q", identifier, got)
		}
	}
	if got := CanonicalIdentifier("Unknown"); got != "Unknown" {
		t.Fatal("unknown app changed")
	}
}
