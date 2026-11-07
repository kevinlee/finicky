package browser

import (
	"fmt"
	"strings"
)

// ValidateProfileDirectory checks an explicit target without opening a browser.
func ValidateProfileDirectory(config BrowserConfig) error {
	if config.ProfileDirectory == "" {
		return nil
	}
	validation := config
	validation.URL = "https://example.com/"
	if _, found, err := pwaOpenArgs(validation, false); found || err != nil {
		return err
	}
	_, err := directProfileArgs(config.Name, config.ProfileDirectory)
	return err
}

// directProfileArgs intentionally does not read Local State. A user-selected
// directory is stable even when names are duplicated or browser data is private.
func directProfileArgs(identifier, directory string) ([]string, error) {
	if directory == "" || strings.TrimSpace(directory) == "" || directory == "." || directory == ".." || strings.ContainsAny(directory, "/\\\x00\r\n") {
		return nil, fmt.Errorf("profile directory must be a single folder name")
	}
	entry := lookupBrowser(identifier)
	if entry == nil {
		return nil, fmt.Errorf("cannot use a profile directory with unrecognized browser %q", identifier)
	}
	if entry.Type != "Chromium" {
		return nil, fmt.Errorf("direct profile directories are supported for Chromium browsers only")
	}
	return []string{"--profile-directory=" + directory}, nil
}
