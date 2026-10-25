// Package urlrequest decodes the shared URL entry point used by extensions,
// Services, Shortcuts and the command line. It never launches applications.
package urlrequest

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
)

type Request struct {
	Focus            string
	Action           string
	URL              string
	Browser          string
	Profile          string
	ProfileDirectory string
	Prompt           bool
	Private          bool
	Background       bool
	NewWindow        bool
	// SourcePrivacy is caller-supplied browser metadata, not authenticated proof.
	// Unknown and private sources must be excluded from learning.
	SourcePrivacy string
}

func Parse(raw string) (Request, error) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil {
		return Request{}, err
	}
	if u.Scheme != "finicky" && u.Scheme != "finicky-plus" {
		if !validURL(u) {
			return Request{}, fmt.Errorf("expected an http, https, or file URL")
		}
		return Request{Action: "open", URL: raw, SourcePrivacy: "unknown"}, nil
	}
	if u.Host == "open" && strings.HasPrefix(u.Path, "/") && len(u.Path) > 1 {
		encoded := strings.TrimPrefix(u.Path, "/")
		var decoded []byte
		for _, encoding := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
			decoded, err = encoding.DecodeString(encoded)
			if err == nil {
				break
			}
		}
		if err != nil {
			return Request{}, fmt.Errorf("invalid encoded URL: %w", err)
		}
		target, parseErr := url.Parse(string(decoded))
		if parseErr != nil || !webURL(target) {
			return Request{}, fmt.Errorf("encoded request must contain a web URL")
		}
		return Request{Action: "open", URL: string(decoded), SourcePrivacy: "unknown"}, nil
	}
	action := u.Host
	if action == "" {
		action = strings.TrimPrefix(u.Opaque, "//")
		if action == "" {
			action = strings.Trim(u.Path, "/")
		}
	}
	q, queryErr := url.ParseQuery(u.RawQuery)
	flag := func(key string) bool { return q.Has(key) && q.Get(key) != "false" && q.Get(key) != "0" }
	r := Request{Action: action, Focus: q.Get("focus"), URL: q.Get("url"), Browser: q.Get("browser"), Profile: q.Get("profile"), ProfileDirectory: q.Get("profileDirectory"), Prompt: flag("prompt"), Private: flag("private"), Background: flag("background"), NewWindow: flag("newWindow"), SourcePrivacy: "unknown"}
	if values := q["sourcePrivacy"]; queryErr == nil && len(values) == 1 && (values[0] == "public" || values[0] == "private") {
		r.SourcePrivacy = values[0]
	}
	if r.Browser == "" {
		r.Browser = q.Get("app")
	}
	switch action {
	case "open", "copy", "share":
		target, e := url.Parse(r.URL)
		if e != nil || !webURL(target) {
			return Request{}, fmt.Errorf("a valid http or https URL is required")
		}
	case "set-default":
		if r.Focus != "clear" && strings.TrimSpace(r.Browser) == "" {
			return Request{}, fmt.Errorf("set-default requires a browser")
		}
	case "clipboard", "settings":
	default:
		return Request{}, fmt.Errorf("unknown Finicky action %q", action)
	}
	if r.ProfileDirectory != "" && (r.ProfileDirectory == "." || r.ProfileDirectory == ".." || strings.ContainsAny(r.ProfileDirectory, "/\\\x00\r\n")) {
		return Request{}, fmt.Errorf("invalid profile directory")
	}
	return r, nil
}

func validURL(u *url.URL) bool {
	return webURL(u) || u != nil && u.Scheme == "file" && u.Path != ""
}

func webURL(u *url.URL) bool {
	return u != nil && (u.Scheme == "https" || u.Scheme == "http") && u.Hostname() != ""
}

// Extract accepts a URL per line or links embedded in selected plain text.
func Extract(text string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, item := range strings.Fields(text) {
		item = trimURLProse(item)
		u, err := url.Parse(item)
		if err == nil && validURL(u) && !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	return result
}

// trimURLProse removes surrounding prose punctuation without truncating
// balanced delimiters that belong to a URL (including an IPv6 host's brackets).
// Percent-encoded delimiters are left encoded and do not affect balancing.
// This is only an extraction heuristic; Parse preserves an exact supplied URL.
func trimURLProse(item string) string {
	item = strings.TrimLeft(item, "<>\"'()[]{}.,;")
	var balance [3]int
	for i := 0; i < len(item); i++ {
		switch item[i] {
		case '(':
			balance[0]++
		case ')':
			balance[0]--
		case '[':
			balance[1]++
		case ']':
			balance[1]--
		case '{':
			balance[2]++
		case '}':
			balance[2]--
		}
	}
	for len(item) > 0 {
		index := -1
		switch item[len(item)-1] {
		case ')':
			index = 0
		case ']':
			index = 1
		case '}':
			index = 2
		case '>', '"', '\'', '.', ',', ';':
			// These delimit a link in ordinary prose.
		default:
			return item
		}
		if index >= 0 {
			if balance[index] >= 0 {
				return item
			}
			balance[index]++
		}
		item = item[:len(item)-1]
	}
	return item
}
