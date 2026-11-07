package linkfeatures

import (
	"finicky/rules"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestPreparePreservesMeaningfulParametersAndEncoding(t *testing.T) {
	no := false
	f := &rules.Features{StripTracking: true, ExpandShortURLs: &no}
	got, err := PrepareURL("https://example.com/path?utm_source=mail&id=1&ref=friend&sig=a%2Fb%20c&fbclid=123#section", f)
	want := "https://example.com/path?id=1&ref=friend&sig=a%2Fb%20c#section"
	if err != nil || got != want {
		t.Fatalf("got=%s err=%v", got, err)
	}
	raw := "https://example.com/?a=1&utm_source=mail"
	got, err = PrepareURL(raw, &rules.Features{ExpandShortURLs: &no})
	if err != nil || got != raw {
		t.Fatal("disabled tracking cleanup changed URL")
	}
}

func TestUnwrapKnownRedirectsAndRejectLookalikes(t *testing.T) {
	no := false
	f := &rules.Features{UnwrapRedirects: true, StripTracking: true, ExpandShortURLs: &no}
	target := "https://example.com/report?utm_source=email&id=10"
	for _, wrapper := range []string{
		"https://www.google.com/url?q=", "https://na01.safelinks.protection.outlook.com/?url=", "https://l.facebook.com/l.php?u=", "https://slack-redir.net/link?url=", "https://www.youtube.com/redirect?q=", "https://duckduckgo.com/l/?uddg=",
	} {
		got, err := PrepareURL(wrapper+url.QueryEscape(target), f)
		if err != nil || got != "https://example.com/report?id=10" {
			t.Errorf("wrapper=%s got=%s err=%v", wrapper, got, err)
		}
	}
	for _, raw := range []string{
		"https://evilgoogle.com/url?q=https%3A%2F%2Fexample.com",
		"https://www.google.com/url?q=javascript%3Aalert(1)",
		"https://www.google.com/url?q=https%3A%2F%2Fuser%3Apass%40example.com",
	} {
		got, err := PrepareURL(raw, f)
		if err != nil || got != raw {
			t.Errorf("unsafe unwrap: %s -> %s err=%v", raw, got, err)
		}
	}
}

func TestPrepareNestedRedirectsAndNonWebSchemes(t *testing.T) {
	no := false
	f := &rules.Features{UnwrapRedirects: true, ExpandShortURLs: &no}
	inner := "https://www.google.com/url?q=" + url.QueryEscape("https://example.com")
	outer := "https://slack-redir.net/link?url=" + url.QueryEscape(inner)
	got, err := PrepareURL(outer, f)
	if err != nil || got != "https://example.com" {
		t.Fatalf("nested unwrap=%s err=%v", got, err)
	}
	for _, raw := range []string{"mailto:friend@example.com", "spotify:track:abc"} {
		got, err := PrepareURL(raw, f)
		if err != nil || got != raw {
			t.Errorf("nonweb URL changed: %s %v", got, err)
		}
	}
}

func TestNativeAppAvailabilityAndExactHosts(t *testing.T) {
	for _, test := range []struct{ raw, app, prefix string }{
		{"https://us02web.zoom.us/j/123456789?pwd=abc", "us.zoom.xos", "zoommtg://us02web.zoom.us/join?"},
		{"https://teams.microsoft.com/l/meetup-join/abc?context=xyz", "com.microsoft.teams2", "msteams://teams.microsoft.com/l/"},
		{"https://open.spotify.com/intl-en/track/abc123?si=xyz", "com.spotify.client", "spotify:track:abc123"},
		{"https://linear.app/acme/issue/ABC-1/example", "Linear", "linear://linear.app/acme/issue/ABC-1"},
		{"https://www.figma.com/design/abc/Example?node-id=1-2", "com.figma.Desktop", "https://www.figma.com/design/abc/Example"},
		{"obsidian://open?vault=Research&file=Paper", "md.obsidian", "obsidian://open?vault=Research"},
		{"https://meet.google.com/abc-defg-hij?authuser=1", "com.google.Chrome", "https://meet.google.com/abc-defg-hij?authuser=1"},
		{"https://app.clickup.com/t/abc?comment=xyz#section", "ClickUp", "clickup://t/abc?comment=xyz#section"},
		{"https://app.slack.com/client/T123/C456", "com.tinyspeck.slackmacgap", "slack://channel?id=C456&team=T123"},
		{"https://slack.com/app_redirect?team=T123&app=A456", "com.tinyspeck.slackmacgap", "slack://app?id=A456&team=T123"},
		{"https://mastodon.social/@user/12345", "Ivory", "ivory:///openURL?url=https%3A%2F%2Fmastodon.social"},
	} {
		got, ok := nativeDestination(test.raw, func(id string) bool { return id == test.app })
		if !ok || got.Name != test.app || !strings.HasPrefix(got.URL, test.prefix) {
			t.Errorf("native mapping: %#v %v", got, ok)
		}
		if _, ok := nativeDestination(test.raw, func(string) bool { return false }); ok {
			t.Fatal("missing app did not fall back")
		}
	}
	for _, raw := range []string{"https://evilzoom.us/j/123", "https://zoom.us.evil.test/j/123", "https://zoom.us/pricing", "https://open.spotify.com/account", "https://teams.microsoft.com/signin", "https://slack.com/app_redirect?channel=C123", "https://slack.com/app_redirect?team=T123&channel=general", "https://acme.slack.com/archives/C123", "https://app.slack.com/client/T123/C456/1234.5678", "https://notmastodon.example/@user/12345", "https://mastodon.social/oauth/authorize", "https://app.clickup.com/login", "https://evilmeet.google.com/abc-defg-hij"} {
		if _, ok := nativeDestination(raw, func(string) bool { return true }); ok {
			t.Fatalf("unsupported native link: %s", raw)
		}
	}
}

func TestNativeServiceFilteringPreservesAllNoneAndSubset(t *testing.T) {
	installed := func(string) bool { return true }
	link := "https://open.spotify.com/track/abc123"
	for _, services := range [][]string{nil, {"spotify"}, {"zoom", "spotify"}} {
		if _, ok := nativeDestinationWithServices(link, services, installed); !ok {
			t.Fatalf("enabled service ignored: %#v", services)
		}
	}
	for _, services := range [][]string{{}, {"zoom"}, {"unknown"}} {
		if _, ok := nativeDestinationWithServices(link, services, installed); ok {
			t.Fatalf("disabled service enabled: %#v", services)
		}
	}
	if _, ok := nativeDestinationWithServices("obsidian://open?vault=Research", []string{}, installed); ok {
		t.Fatal("Obsidian bypassed service filter")
	}
}

func TestHistoryCappedPrivatePermissionsAndClear(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	store := NewStore(path)
	for i := range 205 {
		if err := store.Add(HistoryEntry{URL: fmt.Sprintf("https://example.com/%d", i), Browser: "Chrome"}); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := store.List()
	if err != nil || len(entries) != 200 {
		t.Fatalf("entries=%d err=%v", len(entries), err)
	}
	if entries[0].URL != "https://example.com/204" || entries[199].URL != "https://example.com/5" || entries[0].ID == "" || entries[0].Timestamp.IsZero() {
		t.Fatal("history order or metadata wrong")
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0600 {
		t.Fatalf("history permissions=%o", info.Mode().Perm())
	}
	if err := store.Clear(); err != nil {
		t.Fatal(err)
	}
	entries, err = store.List()
	if err != nil || len(entries) != 0 {
		t.Fatal("history did not clear")
	}
}

func TestHistoryConcurrentAdds(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	store := NewStore(path)
	var wg sync.WaitGroup
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := NewStore(path).Add(HistoryEntry{URL: "https://example.com"}); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	entries, err := store.List()
	if err != nil || len(entries) != 12 {
		t.Fatalf("lost concurrent updates: %d %v", len(entries), err)
	}
}
