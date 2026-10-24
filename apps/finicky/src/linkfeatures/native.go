package linkfeatures

import (
	"finicky/browser"
	"net/url"
	"regexp"
	"strings"
)

// NativeDestination routes only recognized content links, and only when their
// native application is installed. Returning false keeps normal browser routing.
func NativeDestination(raw string) (browser.BrowserConfig, bool) {
	return nativeDestination(raw, browser.AppExists)
}

// NativeDestinationWithServices applies an optional service allowlist. A nil
// list preserves the all-services default; an empty list disables every adapter.
func NativeDestinationWithServices(raw string, services []string) (browser.BrowserConfig, bool) {
	return nativeDestinationWithServices(raw, services, browser.AppExists)
}

func nativeDestination(raw string, installed func(string) bool) (browser.BrowserConfig, bool) {
	return nativeDestinationWithServices(raw, nil, installed)
}

func nativeDestinationWithServices(raw string, services []string, installed func(string) bool) (browser.BrowserConfig, bool) {
	enabled := func(service string) bool {
		if services == nil {
			return true
		}
		for _, value := range services {
			if value == service {
				return true
			}
		}
		return false
	}
	u, err := url.Parse(raw)
	if err != nil || u.User != nil {
		return browser.BrowserConfig{}, false
	}
	// Obsidian Publish URLs do not identify a local vault. Preserve explicit
	// read-only Obsidian URIs instead of inventing a vault mapping.
	// https://obsidian.md/help/uri
	if u.Scheme == "obsidian" && (u.Host == "open" || u.Host == "search" || u.Host == "vault" || u.Host == "choose-vault") {
		if enabled("obsidian") && installed("md.obsidian") {
			return browser.BrowserConfig{Name: "md.obsidian", AppType: "bundleId", URL: raw}, true
		}
		return browser.BrowserConfig{}, false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return browser.BrowserConfig{}, false
	}
	host := strings.ToLower(u.Hostname())
	var apps []string
	var target string
	var service string
	switch {
	// Google Meet is a web application. Keep its URL intact and use Chrome
	// when installed, matching the routing option described by Velja.
	case host == "meet.google.com":
		service, target, apps = "googleMeet", raw, []string{"com.google.Chrome"}
	// ClickUp's documented desktop conversion moves the web path into the
	// custom URI. Scope this to its application host, not marketing pages.
	// https://sindresorhus.com/velja#clickup-with-a-custom-workspace-subdomain
	case host == "app.clickup.com" && u.Path != "" && u.Path != "/" && !strings.HasPrefix(u.Path, "/login") && !strings.HasPrefix(u.Path, "/auth"):
		service, apps = "clickup", []string{"ClickUp"}
		target = "clickup://" + strings.TrimPrefix(u.EscapedPath(), "/")
		if u.RawQuery != "" {
			target += "?" + u.RawQuery
		}
		if u.Fragment != "" {
			target += "#" + u.EscapedFragment()
		}
	// Slack only accepts actual workspace/channel IDs. Never infer an ID
	// from a company subdomain or fall back to a different signed-in team.
	// https://api.slack.com/reference/deep-linking
	case host == "app.slack.com" || host == "slack.com":
		var ok bool
		target, ok = slackDestination(u)
		if !ok {
			return browser.BrowserConfig{}, false
		}
		service, apps = "slack", []string{"com.tinyspeck.slackmacgap"}
	// No network probe or arbitrary-host guessing: these two servers are
	// operated by Mastodon. Other servers can be handled by a custom rule.
	// https://tapbots.com/support/ivory/tips/urlschemes
	case (host == "mastodon.social" || host == "mastodon.online") && regexp.MustCompile(`^/@[A-Za-z0-9_]+(?:/[0-9]+)?/?$`).MatchString(u.Path):
		service, apps = "mastodon", []string{"Ivory"}
		target = "ivory:///openURL?url=" + url.QueryEscape(raw)
	// Linear documents replacing just the scheme, retaining linear.app.
	// https://linear.app/docs/get-the-app
	case host == "linear.app" && strings.Contains(u.Path, "/issue/"):
		service = "linear"
		u.Scheme = "linear"
		target = u.String()
		apps = []string{"Linear"}
	// Figma accepts its ordinary HTTPS content URLs via its native app.
	// https://github.com/johnste/finicky/wiki/Configuration-ideas#open-figma-links-in-figma-app
	case (host == "www.figma.com" || host == "figma.com") && regexp.MustCompile(`^/(file|design|board|proto|slides|make)/[^/]+`).MatchString(u.Path):
		service = "figma"
		target = raw
		apps = []string{"com.figma.Desktop"}
	case domainIs(host, "zoom.us") && regexp.MustCompile(`^/j/[0-9]+/?$`).MatchString(u.Path):
		service = "zoom"
		query := u.Query()
		query.Set("action", "join")
		query.Set("confno", strings.TrimSuffix(strings.TrimPrefix(u.Path, "/j/"), "/"))
		target = (&url.URL{Scheme: "zoommtg", Host: u.Host, Path: "/join", RawQuery: query.Encode()}).String()
		apps = []string{"us.zoom.xos"}
	case (host == "teams.microsoft.com" || host == "teams.live.com") && strings.HasPrefix(u.Path, "/l/"):
		service = "teams"
		u.Scheme = "msteams"
		target = u.String()
		apps = []string{"com.microsoft.teams2", "com.microsoft.teams"}
	case host == "open.spotify.com":
		service = "spotify"
		segments := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(segments) == 3 && strings.HasPrefix(segments[0], "intl-") {
			segments = segments[1:]
		}
		if len(segments) != 2 || !regexp.MustCompile(`^[A-Za-z0-9]+$`).MatchString(segments[1]) {
			return browser.BrowserConfig{}, false
		}
		switch segments[0] {
		case "track", "album", "artist", "playlist", "episode", "show":
			target = "spotify:" + segments[0] + ":" + segments[1]
		default:
			return browser.BrowserConfig{}, false
		}
		apps = []string{"com.spotify.client"}
	default:
		return browser.BrowserConfig{}, false
	}
	if !enabled(service) {
		return browser.BrowserConfig{}, false
	}
	for _, id := range apps {
		if installed(id) {
			appType := "bundleId"
			if id == "Linear" || id == "ClickUp" || id == "Ivory" {
				appType = "appName"
			}
			return browser.BrowserConfig{Name: id, AppType: appType, URL: target}, true
		}
	}
	return browser.BrowserConfig{}, false
}

func slackDestination(u *url.URL) (string, bool) {
	team, kind, id := "", "", ""
	if u.Hostname() == "app.slack.com" {
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) == 3 && parts[0] == "client" {
			team, kind, id = parts[1], "channel", parts[2]
		}
	} else if u.Path == "/app_redirect" {
		team = u.Query().Get("team")
		if channel := u.Query().Get("channel"); channel != "" {
			kind, id = "channel", channel
		} else {
			kind, id = "app", u.Query().Get("app")
		}
	}
	if !regexp.MustCompile(`^[TE][A-Z0-9]+$`).MatchString(team) {
		return "", false
	}
	if kind == "channel" && !regexp.MustCompile(`^[CGD][A-Z0-9]+$`).MatchString(id) {
		return "", false
	}
	if kind == "app" && !regexp.MustCompile(`^A[A-Z0-9]+$`).MatchString(id) {
		return "", false
	}
	if kind == "" {
		return "", false
	}
	return "slack://" + kind + "?" + (url.Values{"team": {team}, "id": {id}}).Encode(), true
}
