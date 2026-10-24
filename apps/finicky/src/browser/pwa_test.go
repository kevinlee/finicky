package browser

import (
	"finicky/util"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const pwaTestID = "abcdefghijklmnopabcdefghijklmnop"

func TestBrowserHomeRelativeAppPath(t *testing.T) {
	home, err := util.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	got, err := expandBrowserAppPath("~/Applications/Example.app")
	if err != nil || got != filepath.Join(home, "Applications/Example.app") {
		t.Fatalf("home expansion: %q %v", got, err)
	}
	args, err := buildOpenArgs(BrowserConfig{Name: "~/Applications/Native Example.app", AppType: "path", URL: "https://example.com"}, false)
	if err != nil || len(args) < 2 || args[1] != filepath.Join(home, "Applications/Native Example.app") {
		t.Fatalf("launcher home expansion: %v %v", args, err)
	}
	if _, err := buildOpenArgs(BrowserConfig{Name: "~/Applications/Finicky Plus.app", URL: "https://example.com"}, false); err == nil {
		t.Fatal("home-relative router target accepted")
	}
}

func pwaFixtureInfo() pwaInfo {
	return pwaInfo{BrowserID: "com.google.Chrome", AppID: pwaTestID, Profile: "Profile 2", DataDir: "/Users/example/Library/Application Support/Google/Chrome/Profile 2/Web Applications/_crx_" + pwaTestID}
}

func TestPWAOpenArgsPreserveModesWithoutRegularTab(t *testing.T) {
	config := BrowserConfig{Name: "/Users/example/Applications/Example.app", URL: "https://example.com/path?a=1&b=2", NewWindow: true,
		Args: []string{"-n", "--args", "--disable-extensions", "https://example.com/path?a=1&b=2"}}
	got, err := buildPWAOpenArgs(config, pwaFixtureInfo(), "/Users/example", true)
	want := []string{"-b", "com.google.Chrome", "-g", "-n", "--args", "--profile-directory=Profile 2", "--new-window", "--disable-extensions", "--app-id=" + pwaTestID, "--app-launch-url-for-shortcuts-menu-item=" + config.URL}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("args=%#v err=%v want=%#v", got, err, want)
	}
	bg := false
	config.OpenInBackground = &bg
	got, err = buildPWAOpenArgs(config, pwaFixtureInfo(), "/Users/example", true)
	if err != nil || strings.Contains(strings.Join(got, "|"), "|-g|") {
		t.Fatalf("explicit foreground ignored: %v %v", got, err)
	}
}

func TestPWARejectsWrongIdentityAndProfile(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*BrowserConfig, *pwaInfo)
	}{
		{"private", func(c *BrowserConfig, _ *pwaInfo) { c.Private = true }},
		{"unknown parent", func(_ *BrowserConfig, p *pwaInfo) { p.BrowserID = "com.other.browser" }},
		{"non-Chromium parent", func(_ *BrowserConfig, p *pwaInfo) { p.BrowserID = "org.mozilla.firefox" }},
		{"name instead of bundle", func(_ *BrowserConfig, p *pwaInfo) { p.BrowserID = "Google Chrome" }},
		{"invalid id", func(_ *BrowserConfig, p *pwaInfo) { p.AppID = strings.Repeat("x", 32) }},
		{"wrong profile", func(c *BrowserConfig, _ *pwaInfo) { c.ProfileDirectory = "Default" }},
		{"traversal", func(_ *BrowserConfig, p *pwaInfo) { p.Profile = "../Profile 2" }},
		{"missing profile", func(_ *BrowserConfig, p *pwaInfo) { p.Profile = "" }},
		{"custom data", func(_ *BrowserConfig, p *pwaInfo) { p.DataDir = "/tmp/Chrome/Profile 2/Web Applications/app" }},
		{"different home", func(_ *BrowserConfig, p *pwaInfo) {
			p.DataDir = strings.ReplaceAll(p.DataDir, "/Users/example", "/Users/another")
		}},
		{"wrong data profile", func(_ *BrowserConfig, p *pwaInfo) { p.DataDir = strings.ReplaceAll(p.DataDir, "Profile 2", "Default") }},
		{"file URL", func(c *BrowserConfig, _ *pwaInfo) { c.URL = "file:///tmp/index.html" }},
		{"custom private", func(c *BrowserConfig, _ *pwaInfo) { c.Args = []string{"--incognito"} }},
		{"custom profile", func(c *BrowserConfig, _ *pwaInfo) { c.Args = []string{"--profile-directory=Default"} }},
		{"extra URL", func(c *BrowserConfig, _ *pwaInfo) { c.Args = []string{"https://another.example/"} }},
	} {
		t.Run(test.name, func(t *testing.T) {
			config := BrowserConfig{URL: "https://example.com"}
			info := pwaFixtureInfo()
			test.change(&config, &info)
			if args, err := buildPWAOpenArgs(config, info, "/Users/example", false); err == nil {
				t.Fatalf("accepted unsafe target: %v", args)
			}
		})
	}
}

func TestPWAMultiProfileRequiresExplicitDirectory(t *testing.T) {
	info := pwaFixtureInfo()
	info.Profile = ""
	info.DataDir = strings.ReplaceAll(info.DataDir, "Profile 2", "-")
	args, err := buildPWAOpenArgs(BrowserConfig{URL: "https://example.com", ProfileDirectory: "Profile 2"}, info, "/Users/example", false)
	if err != nil || !strings.Contains(strings.Join(args, "|"), "--profile-directory=Profile 2") {
		t.Fatalf("multi-profile launch: %v %v", args, err)
	}
}

func TestPWAReadsXMLAndBinaryPlists(t *testing.T) {
	xml := fmt.Sprintf(`<?xml version="1.0"?><plist version="1.0"><dict>
<key>CrBundleIdentifier</key><string>com.google.Chrome</string>
<key>CrAppModeShortcutID</key><string>%s</string>
<key>CrAppModeProfileDir</key><string>Profile 2</string>
</dict></plist>`, pwaTestID)
	path := filepath.Join(t.TempDir(), "Info.plist")
	if err := os.WriteFile(path, []byte(xml), 0600); err != nil {
		t.Fatal(err)
	}
	for _, format := range []string{"xml1", "binary1"} {
		if out, err := exec.Command("/usr/bin/plutil", "-convert", format, path).CombinedOutput(); err != nil {
			t.Fatalf("plist fixture: %v %s", err, out)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		info, found, err := parsePWAInfo(data)
		if err != nil || !found || info.AppID != pwaTestID || info.Profile != "Profile 2" || info.BrowserID != "com.google.Chrome" {
			t.Fatalf("%s: %#v %v %v", format, info, found, err)
		}
	}
	_, found, err := parsePWAInfo([]byte(`<plist><dict><key>CrAppModeShortcutID</key><integer>42</integer></dict></plist>`))
	if !found || err == nil {
		t.Fatal("non-text metadata accepted")
	}
	_, found, err = parsePWAInfo([]byte(`<plist><dict><key>CFBundleIdentifier</key><string>com.example.native</string></dict></plist>`))
	if found || err != nil {
		t.Fatal("ordinary native app identified as web app")
	}
}

func TestPWAWrapperIsRecognizedByLauncher(t *testing.T) {
	app := filepath.Join(t.TempDir(), "Example.app")
	if err := os.MkdirAll(filepath.Join(app, "Contents"), 0700); err != nil {
		t.Fatal(err)
	}
	xml := fmt.Sprintf(`<plist><dict><key>CrBundleIdentifier</key><string>com.google.Chrome</string><key>CrAppModeShortcutID</key><string>%s</string><key>CrAppModeProfileDir</key><string>Default</string></dict></plist>`, pwaTestID)
	if err := os.WriteFile(filepath.Join(app, "Contents", "Info.plist"), []byte(xml), 0600); err != nil {
		t.Fatal(err)
	}
	args, err := buildOpenArgs(BrowserConfig{Name: app, AppType: "path", URL: "https://example.com"}, false)
	if err != nil || len(args) < 2 || args[0] != "-b" || args[1] != "com.google.Chrome" {
		t.Fatalf("launcher did not resolve wrapper: %v %v", args, err)
	}
	if err := ValidateProfileDirectory(BrowserConfig{Name: app, ProfileDirectory: "Default"}); err != nil {
		t.Fatalf("cannot save explicit web-app profile: %v", err)
	}
	if err := ValidateProfileDirectory(BrowserConfig{Name: app, ProfileDirectory: "Profile 2"}); err == nil {
		t.Fatal("can save conflicting web-app profile")
	}
}
