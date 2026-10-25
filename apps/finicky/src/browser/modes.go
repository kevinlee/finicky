package browser

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

func lookupBrowser(identifier string) *browserInfo {
	var entries []browserInfo
	if json.Unmarshal(browsersJsonData, &entries) != nil {
		return nil
	}
	appName := strings.TrimSuffix(filepath.Base(identifier), ".app")
	for _, entry := range entries {
		if entry.ID == identifier || entry.AppName == identifier || (filepath.IsAbs(identifier) && entry.AppName == appName) {
			return &entry
		}
	}
	return nil
}

// CanonicalIdentifier makes app names, bundle IDs, and known .app paths refer
// to the same picker favorite without reading browser profile data.
func CanonicalIdentifier(identifier string) string {
	if entry := lookupBrowser(identifier); entry != nil {
		return entry.ID
	}
	return identifier
}

// BrowserCapabilities allows the chooser to disable unsupported private modes
// instead of claiming privacy and opening a normal window.
type BrowserCapabilities struct {
	Private   bool `json:"private"`
	NewWindow bool `json:"newWindow"`
	Profiles  bool `json:"profiles"`
}

func Capabilities(identifier string) BrowserCapabilities {
	entry := lookupBrowser(identifier)
	if entry == nil {
		return BrowserCapabilities{}
	}
	supported := entry.Type == "Chromium" || entry.Type == "Firefox"
	return BrowserCapabilities{Private: supported, NewWindow: supported, Profiles: supported}
}

func browserModeArgs(config BrowserConfig) ([]string, error) {
	if !config.Private && !config.NewWindow {
		return nil, nil
	}
	entry := lookupBrowser(config.Name)
	if entry == nil {
		return nil, fmt.Errorf("private/new-window launching is not supported for %q", config.Name)
	}
	var args []string
	switch entry.Type {
	case "Chromium":
		if config.Private {
			if strings.HasPrefix(entry.ID, "com.microsoft.edgemac") {
				args = append(args, "--inprivate")
			} else {
				args = append(args, "--incognito")
			}
		}
		if config.NewWindow {
			args = append(args, "--new-window")
		}
	case "Firefox":
		if config.Private {
			args = append(args, "-private-window")
		} else if config.NewWindow {
			args = append(args, "-new-window")
		}
	default:
		return nil, fmt.Errorf("private/new-window launching is not supported for %q", config.Name)
	}
	return args, nil
}

// ValidateTarget includes explicit named-profile resolution and mode support.
func ValidateTarget(config BrowserConfig) error {
	if IsRouter(config.Name) {
		return fmt.Errorf("%q is a link router, not a browser destination", config.Name)
	}
	if config.AppType == "none" {
		return nil
	}
	_, err := buildOpenArgs(config, false)
	return err
}

// IsRouter prevents a default, custom target, or app path from recursively
// handing the same link back to Finicky.
func IsRouter(identifier string) bool {
	if filepath.IsAbs(identifier) {
		identifier = strings.TrimSuffix(filepath.Base(identifier), ".app")
	}
	switch strings.ToLower(identifier) {
	case "waystile", "waystile preview", "finicky", "finicky plus", "finicky plus preview", "se.johnste.finicky", "net.kassett.finicky", "com.kevinlee.finicky", "com.kevinlee.finicky.preview":
		return true
	}
	return false
}
