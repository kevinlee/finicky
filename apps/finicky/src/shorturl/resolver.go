package shorturl

import (
	"embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

//go:embed shortener_domains.json
var shortenerDomainsFS embed.FS

// Common URL shortener domains
var shortenerDomains []string

func init() {
	// Load shortener domains from embedded JSON file
	data, err := shortenerDomainsFS.ReadFile("shortener_domains.json")
	if err != nil {
		slog.Error("Failed to read shortener domains file", "error", err)
		// Fallback to empty list if file cannot be read
		shortenerDomains = []string{}
		return
	}

	if err := json.Unmarshal(data, &shortenerDomains); err != nil {
		slog.Error("Failed to parse shortener domains JSON", "error", err)
		// Fallback to empty list if JSON is invalid
		shortenerDomains = []string{}
	}
}

// ResolveURL resolves a potentially shortened URL to its final destination by following HTTP redirects, so
// the matcher can match the final URL instead of the short URL.
func ResolveURL(originalURL string) (string, error) {
	return ResolveURLWithDomains(originalURL, nil)
}

// ResolveURLWithDomains extends the built-in list without changing global state.
func ResolveURLWithDomains(originalURL string, extraDomains []string) (string, error) {
	parsedURL, err := url.Parse(originalURL)
	if err != nil {
		return originalURL, fmt.Errorf("failed to parse URL: %w", err)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return originalURL, nil
	}
	host := strings.ToLower(strings.TrimSuffix(parsedURL.Hostname(), "."))
	known := func(domains []string) bool {
		for _, domain := range domains {
			if matchesDomain(host, domain) {
				return true
			}
		}
		return false
	}
	if !known(shortenerDomains) && !known(extraDomains) {
		return originalURL, nil
	}
	lastURL := originalURL
	client := &http.Client{
		Timeout: 750 * time.Millisecond,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return fmt.Errorf("unsupported redirect scheme")
			}
			lastURL = req.URL.String()
			if len(via) >= 3 {
				return fmt.Errorf("stopped after 3 redirects")
			}
			return nil
		},
	}
	var lastErr error
	for _, method := range []string{http.MethodHead, http.MethodGet} {
		req, err := http.NewRequest(method, originalURL, nil)
		if err != nil {
			return lastURL, err
		}
		req.Header.Set("User-Agent", "Finicky/4.0")
		resp, err := client.Do(req)
		// Close on every path, including successful HEAD and GET responses.
		if resp != nil {
			if resp.Body != nil {
				resp.Body.Close()
			}
			if err == nil && resp.StatusCode >= 200 && resp.StatusCode < 400 {
				return resp.Request.URL.String(), nil
			}
			if err == nil {
				err = fmt.Errorf("shortener returned HTTP %d", resp.StatusCode)
			}
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("shortener returned no response")
	}
	return lastURL, lastErr
}

func matchesDomain(host, domain string) bool {
	domain = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(domain), "."))
	return domain != "" && (host == domain || strings.HasSuffix(host, "."+domain))
}
