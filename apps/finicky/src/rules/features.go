package rules

import (
	"finicky/browser"
	"fmt"
	"strings"
)

// Features are opt-in link conveniences. A nil ExpandShortURLs preserves
// Finicky's existing automatic short-link expansion behavior.
type Features struct {
	TransformScript string   `json:"transformScript,omitempty"`
	MenuIcon        string   `json:"menuIcon,omitempty"`
	PromptMode      string   `json:"promptMode,omitempty"`
	PromptModifier  string   `json:"promptModifier,omitempty"`
	StripTracking   bool     `json:"stripTracking,omitempty"`
	UnwrapRedirects bool     `json:"unwrapRedirects,omitempty"`
	ExpandShortURLs *bool    `json:"expandShortURLs,omitempty"`
	ExtraShorteners []string `json:"extraShorteners,omitempty"`
	HistoryEnabled  bool     `json:"historyEnabled,omitempty"`
	LearningEnabled bool     `json:"learningEnabled,omitempty"`
	NativeApps      bool     `json:"nativeApps,omitempty"`
	// nil selects all supported services; an explicit empty list selects none.
	NativeAppServices           []string `json:"nativeAppServices"`
	AlternativeBrowser          string   `json:"alternativeBrowser,omitempty"`
	AlternativePrivate          bool     `json:"alternativePrivate,omitempty"`
	AlternativeProfile          string   `json:"alternativeProfile,omitempty"`
	AlternativeProfileDirectory string   `json:"alternativeProfileDirectory,omitempty"`
}

func ValidateFeatures(f *Features) error {
	if f == nil {
		return nil
	}
	switch f.MenuIcon {
	case "", "default", "browser", "globe", "arrow":
	default:
		return fmt.Errorf("unsupported menu icon")
	}
	switch f.PromptMode {
	case "", "never", "always", "unmatched":
	default:
		return fmt.Errorf("prompt mode must be never, always, or unmatched")
	}
	switch f.PromptModifier {
	case "", "none", "fn", "option", "shift", "control", "command":
	default:
		return fmt.Errorf("unsupported prompt modifier %q", f.PromptModifier)
	}
	for _, domain := range f.ExtraShorteners {
		if domain == "" || strings.ContainsAny(domain, "/:@ ?#\\\t\r\n") || strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") {
			return fmt.Errorf("shortener must be a hostname, such as example.com")
		}
	}
	if (f.AlternativeProfile != "" || f.AlternativeProfileDirectory != "") && f.AlternativeBrowser == "" {
		return fmt.Errorf("alternative profile requires a browser")
	}
	return browser.ValidateProfileDirectory(browser.BrowserConfig{Name: f.AlternativeBrowser, ProfileDirectory: f.AlternativeProfileDirectory})
}
