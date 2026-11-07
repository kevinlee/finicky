package rules

import (
	"encoding/json"
	"finicky/browser"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Rule struct {
	Match            []string `json:"match"`
	Browser          string   `json:"browser"`
	Profile          string   `json:"profile,omitempty"`
	ProfileDirectory string   `json:"profileDirectory,omitempty"`
	Name             string   `json:"name,omitempty"`
	SourceApps       []string `json:"sourceApps,omitempty"`
	Enabled          *bool    `json:"enabled,omitempty"`
	Private          bool     `json:"private,omitempty"`
	NewWindow        bool     `json:"newWindow,omitempty"`
	TransformScript  string   `json:"transformScript,omitempty"`
}

// UnmarshalJSON accepts both a single string and an array for the match field.
func (r *Rule) UnmarshalJSON(data []byte) error {
	type alias Rule
	var raw struct {
		Match json.RawMessage `json:"match"`
		*alias
	}
	*r = Rule{}
	raw.alias = (*alias)(r)
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if raw.Match != nil && string(raw.Match) != "null" {
		var s string
		if err := json.Unmarshal(raw.Match, &s); err == nil {
			r.Match = []string{s}
			return nil
		}
		return json.Unmarshal(raw.Match, &r.Match)
	}
	return nil
}

// MarshalJSON serializes match as a plain string when there is only one entry.
func (r Rule) MarshalJSON() ([]byte, error) {
	type alias Rule
	var match interface{}
	if len(r.Match) == 1 {
		match = r.Match[0]
	} else {
		match = r.Match
	}
	return json.Marshal(struct {
		Match interface{} `json:"match"`
		alias
	}{Match: match, alias: alias(r)})
}

// SavedProfile is a UI favorite. Rules retain concrete browser targets so
// renaming or removing a favorite does not silently change existing routing.
type SavedProfile struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Browser          string `json:"browser"`
	Profile          string `json:"profile,omitempty"`
	ProfileDirectory string `json:"profileDirectory,omitempty"`
	Color            string `json:"color,omitempty"`
	Shortcut         string `json:"shortcut,omitempty"`
	Hidden           bool   `json:"hidden,omitempty"`
	Private          bool   `json:"private,omitempty"`
}

type Options struct {
	KeepRunning     *bool `json:"keepRunning,omitempty"`
	HideIcon        *bool `json:"hideIcon,omitempty"`
	LogRequests     *bool `json:"logRequests,omitempty"`
	CheckForUpdates *bool `json:"checkForUpdates,omitempty"`
}

type RulesFile struct {
	DefaultBrowser          string         `json:"defaultBrowser"`
	DefaultProfile          string         `json:"defaultProfile,omitempty"`
	DefaultProfileDirectory string         `json:"defaultProfileDirectory,omitempty"`
	DefaultPrivate          bool           `json:"defaultPrivate,omitempty"`
	Options                 *Options       `json:"options,omitempty"`
	Rules                   []Rule         `json:"rules"`
	Profiles                []SavedProfile `json:"profiles,omitempty"`
	Features                *Features      `json:"features,omitempty"`
}

var customPath string

// SetCustomPath overrides the default rules.json location. Pass an empty
// string to revert to the default. Intended for testing and CLI flags.
func SetCustomPath(path string) {
	customPath = path
}

// GetPath returns the path to the rules JSON file.
// Returns the custom path if one was set via SetCustomPath, otherwise
// ~/Library/Application Support/Finicky Plus/rules.json.
func GetPath() (string, error) {
	if customPath != "" {
		return customPath, nil
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "Finicky Plus", "rules.json"), nil
}

// Load reads the rules file from the default path. Returns an empty RulesFile if it doesn't exist.
func Load() (RulesFile, error) {
	path, err := GetPath()
	if err != nil {
		return RulesFile{}, err
	}
	return LoadFromPath(path)
}

// LoadFromPath reads a rules file from the given path. Returns an empty RulesFile if it doesn't exist.
func LoadFromPath(path string) (RulesFile, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return RulesFile{Rules: []Rule{}}, nil
	}
	if err != nil {
		return RulesFile{}, err
	}

	var rf RulesFile
	if err := json.Unmarshal(data, &rf); err != nil {
		return RulesFile{}, err
	}
	if rf.Rules == nil {
		rf.Rules = []Rule{}
	}
	return rf, nil
}

// Save writes the rules file to the default path, creating the directory if needed.
func Save(rf RulesFile) error {
	path, err := GetPath()
	if err != nil {
		return err
	}
	return SaveToPath(rf, path)
}

// SaveToPath writes the rules file to the given path, creating the directory if needed.
func SaveToPath(rf RulesFile, path string) error {
	mu := pathWriteLock(path)
	mu.Lock()
	defer mu.Unlock()
	return saveToPath(rf, path)
}

// saveToPath requires the caller to hold pathWriteLock(path).
func saveToPath(rf RulesFile, path string) error {
	if err := Validate(rf); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(rf, "", "  ")
	if err != nil {
		return err
	}

	// Keep the last complete version and replace atomically. Neither a failed
	// serialization nor an interrupted write should destroy working rules.
	if previous, readErr := os.ReadFile(path); readErr == nil {
		if err := writeAtomic(path+".backup", previous); err != nil {
			return fmt.Errorf("could not back up rules: %w", err)
		}
	} else if !os.IsNotExist(readErr) {
		return readErr
	}
	return writeAtomic(path, data)
}

func writeAtomic(path string, data []byte) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".finicky-rules-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err = file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err = file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}

// Validate checks explicit directory targets without touching browser data.
func Validate(rf RulesFile) error {
	if err := ValidateFeatures(rf.Features); err != nil {
		return err
	}
	bundleIDPattern := regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9.-]*$`)
	validateDirectory := func(name, value string) error {
		if value != "" && (value == "." || value == ".." || strings.ContainsAny(value, "/\\\x00\r\n") || strings.TrimSpace(value) == "") {
			return fmt.Errorf("profile directory must be a single folder name, such as Default or Profile 2")
		}
		return browser.ValidateProfileDirectory(browser.BrowserConfig{Name: name, ProfileDirectory: value})
	}
	if err := validateDirectory(rf.DefaultBrowser, rf.DefaultProfileDirectory); err != nil {
		return fmt.Errorf("default profile: %w", err)
	}
	for i, rule := range rf.Rules {
		if err := validateDirectory(rule.Browser, rule.ProfileDirectory); err != nil {
			return fmt.Errorf("rule %d: %w", i+1, err)
		}
		for _, sourceApp := range rule.SourceApps {
			if sourceApp = strings.TrimSpace(sourceApp); sourceApp != "" && !bundleIDPattern.MatchString(sourceApp) {
				return fmt.Errorf("rule %d: source app must be a bundle identifier, such as com.tinyspeck.slackmacgap", i+1)
			}
		}
	}
	for _, profile := range rf.Profiles {
		if err := validateDirectory(profile.Browser, profile.ProfileDirectory); err != nil {
			return fmt.Errorf("profile %q: %w", profile.Name, err)
		}
	}
	return nil
}

// ToJSHandlers converts rules to the handler format expected by finickyConfigAPI.
// Rules with an empty match or browser are skipped.
func ToJSHandlers(rules []Rule) []map[string]interface{} {
	handlers := make([]map[string]interface{}, 0, len(rules))
	for index, r := range rules {
		// Filter out empty patterns
		matches := make([]string, 0, len(r.Match))
		for _, m := range r.Match {
			if m != "" {
				matches = append(matches, m)
			}
		}
		sourceApps := make([]string, 0, len(r.SourceApps))
		for _, app := range r.SourceApps {
			if app = strings.TrimSpace(app); app != "" {
				sourceApps = append(sourceApps, app)
			}
		}
		if r.Enabled != nil && !*r.Enabled || r.Browser == "" || len(matches) == 0 && len(sourceApps) == 0 {
			continue
		}
		if len(matches) == 0 {
			matches = []string{"*"}
		}
		var matchVal interface{}
		if len(matches) == 1 {
			matchVal = matches[0]
		} else {
			matchVal = matches
		}
		var browser interface{}
		if r.Profile != "" || r.ProfileDirectory != "" || r.Private || r.NewWindow {
			browser = map[string]interface{}{"name": r.Browser, "profile": r.Profile}
			if r.ProfileDirectory != "" {
				browser.(map[string]interface{})["profileDirectory"] = r.ProfileDirectory
			}
			if r.Private {
				browser.(map[string]interface{})["private"] = true
			}
			if r.NewWindow {
				browser.(map[string]interface{})["newWindow"] = true
			}
		} else {
			browser = r.Browser
		}
		handler := map[string]interface{}{
			"match":     matchVal,
			"browser":   browser,
			"ruleIndex": index,
		}
		if r.TransformScript != "" {
			handler["transformScript"] = r.TransformScript
		}
		if len(sourceApps) > 0 {
			handler["sourceApps"] = sourceApps
		}
		handlers = append(handlers, handler)
	}
	return handlers
}

// ToJSConfigScript generates a JavaScript config assignment for the given namespace.
// It produces a valid finickyConfig object that can be evaluated in the JS VM.
func ToJSConfigScript(rf RulesFile, namespace string) (string, error) {
	defaultBrowser := rf.DefaultBrowser
	if defaultBrowser == "" {
		defaultBrowser = "com.apple.Safari"
	}

	var defaultBrowserObj interface{}
	if rf.DefaultProfile != "" || rf.DefaultProfileDirectory != "" || rf.DefaultPrivate {
		defaultBrowserObj = map[string]interface{}{"name": defaultBrowser, "profile": rf.DefaultProfile, "private": rf.DefaultPrivate}
		if rf.DefaultProfileDirectory != "" {
			defaultBrowserObj.(map[string]interface{})["profileDirectory"] = rf.DefaultProfileDirectory
		}
	} else {
		defaultBrowserObj = defaultBrowser
	}

	defaultBrowserJSON, err := json.Marshal(defaultBrowserObj)
	if err != nil {
		return "", fmt.Errorf("failed to marshal defaultBrowser: %v", err)
	}

	handlersJSON, err := json.Marshal(ToJSHandlers(rf.Rules))
	if err != nil {
		return "", fmt.Errorf("failed to marshal handlers: %v", err)
	}

	if rf.Options == nil && (rf.Features == nil || rf.Features.TransformScript == "") {
		return fmt.Sprintf("var %s = {default: {defaultBrowser: %s, handlers: %s}};",
			namespace, string(defaultBrowserJSON), string(handlersJSON)), nil
	}

	opts := make(map[string]interface{})
	if rf.Features != nil && rf.Features.TransformScript != "" {
		opts["transformScript"] = rf.Features.TransformScript
	}
	if rf.Options != nil {
		if rf.Options.KeepRunning != nil {
			opts["keepRunning"] = *rf.Options.KeepRunning
		}
		if rf.Options.HideIcon != nil {
			opts["hideIcon"] = *rf.Options.HideIcon
		}
		if rf.Options.LogRequests != nil {
			opts["logRequests"] = *rf.Options.LogRequests
		}
		if rf.Options.CheckForUpdates != nil {
			opts["checkForUpdates"] = *rf.Options.CheckForUpdates
		}
	}

	optsJSON, err := json.Marshal(opts)
	if err != nil {
		return "", fmt.Errorf("failed to marshal options: %v", err)
	}

	return fmt.Sprintf("var %s = {default: {defaultBrowser: %s, handlers: %s, options: %s}};",
		namespace, string(defaultBrowserJSON), string(handlersJSON), string(optsJSON)), nil
}
