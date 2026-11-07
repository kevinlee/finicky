package linkfeatures

import (
	"finicky/rules"
	"finicky/shorturl"
	"fmt"
	"net/url"
	"strings"
)

// PrepareURL applies only enabled local transformations, with short-link
// expansion preserving its legacy default unless explicitly disabled.
func PrepareURL(raw string, f *rules.Features) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" {
		return raw, fmt.Errorf("invalid URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return raw, nil
	}
	if parsed.Hostname() == "" {
		return raw, fmt.Errorf("URL has no hostname")
	}
	if f != nil && f.UnwrapRedirects {
		raw = unwrap(raw)
	}
	var expansionErr error
	if f == nil || f.ExpandShortURLs == nil || *f.ExpandShortURLs {
		var extra []string
		if f != nil {
			extra = f.ExtraShorteners
		}
		raw, expansionErr = shorturl.ResolveURLWithDomains(raw, extra)
	}
	if f != nil && f.UnwrapRedirects {
		raw = unwrap(raw)
	}
	if f != nil && f.StripTracking {
		raw = stripTracking(raw)
	}
	return raw, expansionErr
}

func stripTracking(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.RawQuery == "" {
		return raw
	}
	// Preserve encoding, ordering, signatures, and empty fields of parameters
	// that are not known trackers; generic "ref" or "id" fields are meaningful.
	parts := strings.Split(u.RawQuery, "&")
	kept := make([]string, 0, len(parts))
	for _, part := range parts {
		key, _, _ := strings.Cut(part, "=")
		key, err = url.QueryUnescape(key)
		if err != nil {
			kept = append(kept, part)
			continue
		}
		key = strings.ToLower(key)
		tracking := strings.HasPrefix(key, "utm_")
		switch key {
		case "fbclid", "gclid", "dclid", "gbraid", "wbraid", "msclkid", "igshid", "mc_cid", "mc_eid", "yclid", "twclid", "ttclid", "li_fat_id", "_ga", "_gl":
			tracking = true
		}
		if !tracking {
			kept = append(kept, part)
		}
	}
	if len(kept) == len(parts) {
		return raw
	}
	u.RawQuery = strings.Join(kept, "&")
	u.ForceQuery = false
	return u.String()
}

func domainIs(host, domain string) bool {
	return host == domain || strings.HasSuffix(host, "."+domain)
}

func unwrap(raw string) string {
	seen := map[string]bool{}
	for range 5 {
		if seen[raw] {
			break
		}
		seen[raw] = true
		u, err := url.Parse(raw)
		if err != nil {
			break
		}
		host := strings.ToLower(u.Hostname())
		query := u.Query()
		var candidate string
		switch {
		case domainIs(host, "google.com") && u.Path == "/url":
			candidate = query.Get("url")
			if candidate == "" {
				candidate = query.Get("q")
			}
		case (host == "l.facebook.com" || host == "lm.facebook.com") && u.Path == "/l.php", host == "l.instagram.com":
			candidate = query.Get("u")
		case domainIs(host, "safelinks.protection.outlook.com"):
			candidate = query.Get("url")
		case host == "slack-redir.net" && u.Path == "/link":
			candidate = query.Get("url")
		case domainIs(host, "youtube.com") && u.Path == "/redirect":
			candidate = query.Get("q")
		case domainIs(host, "linkedin.com") && u.Path == "/safety/go":
			candidate = query.Get("url")
		case domainIs(host, "duckduckgo.com") && strings.HasPrefix(u.Path, "/l/"):
			candidate = query.Get("uddg")
		}
		target, err := url.Parse(candidate)
		if err != nil || (target.Scheme != "https" && target.Scheme != "http") || target.Hostname() == "" || target.User != nil {
			break
		}
		raw = target.String()
	}
	return raw
}
