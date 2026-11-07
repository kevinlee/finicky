package browser

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"al.essio.dev/pkg/shellescape"
	"finicky/util"
)

//go:embed browsers.json
var browsersJsonData []byte

type BrowserResult struct {
	Browser  BrowserConfig `json:"browser"`
	Error    string        `json:"error"`
	Warnings []string      `json:"warnings"`
}

type BrowserConfig struct {
	Action           string   `json:"action,omitempty"`
	MatchedRule      *int     `json:"matchedRule,omitempty"`
	Name             string   `json:"name"`
	AppType          string   `json:"appType"`
	OpenInBackground *bool    `json:"openInBackground"`
	Profile          string   `json:"profile"`
	ProfileDirectory string   `json:"profileDirectory,omitempty"`
	Private          bool     `json:"private,omitempty"`
	NewWindow        bool     `json:"newWindow,omitempty"`
	Args             []string `json:"args"`
	URL              string   `json:"url"`
}

type browserInfo struct {
	ConfigDirRelative string `json:"config_dir_relative"`
	ID                string `json:"id"`
	AppName           string `json:"app_name"`
	Type              string `json:"type"`
}

func LaunchBrowser(config BrowserConfig, dryRun bool, openInBackgroundByDefault bool) error {
	if config.Action == "prompt" {
		return fmt.Errorf("a browser must be selected before opening this link")
	}
	if config.AppType == "none" {
		slog.Info("AppType is 'none', not launching any browser")
		return nil
	}

	slog.Info("Starting browser", "name", config.Name, "url", config.URL)

	openArgs, err := buildOpenArgs(config, openInBackgroundByDefault)
	if err != nil {
		return err
	}

	cmd := exec.Command("open", openArgs...)

	// Pretty print the command with proper escaping
	prettyCmd := formatCommand(cmd.Path, cmd.Args)

	if dryRun {
		slog.Debug("Would run command (dry run)", "command", prettyCmd)
		return nil
	} else {
		slog.Debug("Run command", "command", prettyCmd)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	stderrBytes, err := io.ReadAll(stderr)
	if err != nil {
		return fmt.Errorf("error reading stderr: %v", err)
	}

	stdoutBytes, err := io.ReadAll(stdout)
	if err != nil {
		return fmt.Errorf("error reading stdout: %v", err)
	}

	cmdErr := cmd.Wait()

	if len(stderrBytes) > 0 {
		slog.Error("Command returned error", "error", string(stderrBytes))
	}
	if len(stdoutBytes) > 0 {
		slog.Debug("Command returned output", "output", string(stdoutBytes))
	}

	if cmdErr != nil {
		return fmt.Errorf("command failed: %v", cmdErr)
	}

	return nil
}

// buildOpenArgs separates target resolution from launching for dry-run tests.
func buildOpenArgs(config BrowserConfig, openInBackgroundByDefault bool) ([]string, error) {
	name, err := expandBrowserAppPath(config.Name)
	if err != nil {
		return nil, err
	}
	config.Name = name
	if IsRouter(config.Name) {
		return nil, fmt.Errorf("%q is a link router, not a browser destination", config.Name)
	}
	if args, found, err := pwaOpenArgs(config, openInBackgroundByDefault); found || err != nil {
		return args, err
	}
	var openArgs []string

	if config.AppType == "bundleId" {
		openArgs = []string{"-b", config.Name}
	} else {
		openArgs = []string{"-a", config.Name}
	}

	var openInBackground bool = openInBackgroundByDefault

	if config.OpenInBackground != nil {
		openInBackground = *config.OpenInBackground
	}

	if openInBackground {
		openArgs = append(openArgs, "-g")
	}

	var browserArgs []string
	if config.ProfileDirectory != "" {
		args, err := directProfileArgs(config.Name, config.ProfileDirectory)
		if err != nil {
			return nil, err
		}
		browserArgs = append(browserArgs, args...)
	} else if config.Profile != "" {
		args, ok := resolveBrowserProfileArgs(config.Name, config.Profile)
		if !ok {
			return nil, fmt.Errorf("could not resolve profile %q for %s; choose an existing profile or explicit directory", config.Profile, config.Name)
		}
		browserArgs = append(browserArgs, args...)
	}
	modeArgs, err := browserModeArgs(config)
	if err != nil {
		return nil, err
	}
	browserArgs = append(browserArgs, modeArgs...)
	needsInstance := len(browserArgs) > 0

	// Legacy custom args may include macOS open flags before an explicit
	// separator. Keep those outside browser arguments, including -n.
	customArgs := config.Args
	if separator := slices.Index(customArgs, "--args"); separator >= 0 {
		openArgs = append(openArgs, customArgs[:separator]...)
		customArgs = customArgs[separator+1:]
	}
	if needsInstance && !slices.Contains(openArgs, "-n") {
		openArgs = append(openArgs, "-n")
	}
	if needsInstance || len(config.Args) > 0 {
		openArgs = append(openArgs, "--args")
		openArgs = append(openArgs, browserArgs...)
		openArgs = append(openArgs, customArgs...)
		// Existing custom args normally contain their URL; avoid opening it twice.
		if len(config.Args) == 0 || (needsInstance && config.URL != "" && !slices.Contains(customArgs, config.URL)) {
			openArgs = append(openArgs, config.URL)
		}
	} else {
		openArgs = append(openArgs, config.URL)
	}

	return openArgs, nil
}

func resolveBrowserProfileArgs(identifier string, profile string) ([]string, bool) {
	matchedBrowser := lookupBrowser(identifier)

	if matchedBrowser == nil {
		return nil, false
	}

	slog.Debug("Browser found in browsers.json", "identifier", identifier, "type", matchedBrowser.Type)

	if profile != "" {
		switch matchedBrowser.Type {
		case "Chromium":
			homeDir, err := util.UserHomeDir()
			if err != nil {
				slog.Info("Error getting home directory", "error", err)
				return nil, false
			}

			localStatePath := filepath.Join(homeDir, "Library/Application Support", matchedBrowser.ConfigDirRelative, "Local State")
			profilePath, ok := parseProfiles(localStatePath, profile)
			if ok {
				return []string{"--profile-directory=" + profilePath}, true
			}
		case "Firefox":
			homeDir, err := util.UserHomeDir()
			if err != nil {
				slog.Info("Error getting home directory", "error", err)
				return nil, false
			}

			return resolveFirefoxProfileArgs(filepath.Join(homeDir, "Library/Application Support", matchedBrowser.ConfigDirRelative), profile)

		default:
			slog.Info("Browser is not a supported browser type, skipping profile detection", "identifier", identifier)
		}
	}

	return nil, false
}

func readFirefoxProfileNames(profilesIniPath string) []string {
	data, err := os.ReadFile(profilesIniPath)
	if err != nil {
		slog.Info("Error reading profiles.ini", "path", profilesIniPath, "error", err)
		return []string{}
	}

	names := []string{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if name, ok := strings.CutPrefix(line, "Name="); ok {
			names = append(names, name)
		}
	}
	return names
}

func parseFirefoxProfiles(profilesIniPath string, profile string) (string, bool) {
	names := readFirefoxProfileNames(profilesIniPath)
	matches := 0
	for _, name := range names {
		if name == profile {
			matches++
		}
	}
	if matches == 1 {
		return profile, true
	}
	if matches > 1 {
		slog.Warn("Ambiguous Firefox profile name", "profile", profile)
		return "", false
	}
	slog.Warn("Could not find profile in Firefox profiles.", "Expected profile", profile, "Available profiles", strings.Join(names, ", "))
	return "", false
}

func chromiumInfoCache(localStatePath string) (map[string]interface{}, bool) {
	data, err := os.ReadFile(localStatePath)
	if err != nil {
		slog.Info("Error reading Local State file", "path", localStatePath, "error", err)
		return nil, false
	}

	var localState map[string]interface{}
	if err := json.Unmarshal(data, &localState); err != nil {
		slog.Info("Error parsing Local State JSON", "error", err)
		return nil, false
	}

	profiles, ok := localState["profile"].(map[string]interface{})
	if !ok {
		slog.Info("Could not find profile section in Local State")
		return nil, false
	}

	infoCache, ok := profiles["info_cache"].(map[string]interface{})
	if !ok {
		slog.Info("Could not find info_cache in profile section")
		return nil, false
	}

	return infoCache, true
}

func getAllChromiumProfiles(localStatePath string) []string {
	cache, ok := chromiumInfoCache(localStatePath)
	if !ok {
		return []string{}
	}

	var names []string
	for _, info := range cache {
		profileInfo, ok := info.(map[string]interface{})
		if !ok {
			continue
		}
		name, ok := profileInfo["name"].(string)
		if !ok {
			continue
		}
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func parseProfiles(localStatePath string, profile string) (string, bool) {
	infoCache, ok := chromiumInfoCache(localStatePath)
	if !ok {
		return "", false
	}
	// Folder IDs remain stable across renames and duplicate display names.
	if _, exists := infoCache[profile]; exists {
		return profile, true
	}
	var matched string
	for directory, info := range infoCache {
		entry, valid := info.(map[string]interface{})
		if !valid || entry["name"] != profile {
			continue
		}
		if matched != "" {
			slog.Warn("Ambiguous browser profile name; use a profile directory", "profile", profile)
			return "", false
		}
		matched = directory
	}
	return matched, matched != ""
}

// GetProfilesForBrowser returns available profile names for a given browser app name or bundle ID.
// Returns empty slice if browser not in browsers.json, not supported, or profile files are unreadable.
func GetProfilesForBrowser(identifier string) []string {
	var browsersJson []browserInfo
	if err := json.Unmarshal(browsersJsonData, &browsersJson); err != nil {
		slog.Info("Error parsing browsers.json", "error", err)
		return []string{}
	}

	var matchedBrowser *browserInfo
	for i := range browsersJson {
		if browsersJson[i].ID == identifier || browsersJson[i].AppName == identifier {
			matchedBrowser = &browsersJson[i]
			break
		}
	}

	if matchedBrowser == nil {
		return []string{}
	}

	homeDir, err := util.UserHomeDir()
	if err != nil {
		slog.Info("Error getting home directory", "error", err)
		return []string{}
	}

	switch matchedBrowser.Type {
	case "Chromium":
		localStatePath := filepath.Join(homeDir, "Library/Application Support", matchedBrowser.ConfigDirRelative, "Local State")
		return getAllChromiumProfiles(localStatePath)
	case "Firefox":
		var names []string
		for _, profile := range readFirefoxProfileRecords(filepath.Join(homeDir, "Library/Application Support", matchedBrowser.ConfigDirRelative)) {
			names = append(names, profile.name)
		}
		return names

	default:
		return []string{}
	}
}

// formatCommand returns a properly shell-escaped string representation of the command
func formatCommand(path string, args []string) string {
	if len(args) == 0 {
		return shellescape.Quote(path)
	}

	quotedArgs := make([]string, len(args))
	for i, arg := range args {
		quotedArgs[i] = shellescape.Quote(arg)
	}

	return strings.Join(quotedArgs, " ")
}
