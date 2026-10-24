package browser

/*
#cgo LDFLAGS: -framework Foundation
#include <stdlib.h>
char *finickyPWAInfoJSON(const void *bytes, size_t length);
*/
import "C"

import (
	"encoding/json"
	"finicky/util"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unsafe"
)

type pwaInfo struct {
	BrowserID string `json:"CrBundleIdentifier"`
	AppID     string `json:"CrAppModeShortcutID"`
	Profile   string `json:"CrAppModeProfileDir"`
	DataDir   string `json:"CrAppModeUserDataDir"`
	Error     string `json:"error"`
}

var pwaIDPattern = regexp.MustCompile(`^[a-p]{32}$`)

// pwaOpenArgs recognizes an existing Chromium web-app wrapper selected by path.
// All other app destinations continue through the ordinary launcher.
func pwaOpenArgs(config BrowserConfig, background bool) ([]string, bool, error) {
	name, err := expandBrowserAppPath(config.Name)
	if err != nil {
		return nil, true, err
	}
	config.Name = name
	if !filepath.IsAbs(config.Name) || !strings.EqualFold(filepath.Ext(config.Name), ".app") {
		return nil, false, nil
	}
	file, err := os.Open(filepath.Join(config.Name, "Contents", "Info.plist"))
	if err != nil {
		return nil, false, nil
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, (1<<20)+1))
	if err != nil || len(data) > 1<<20 {
		return nil, true, fmt.Errorf("could not read web app metadata from %q", config.Name)
	}
	info, found, err := parsePWAInfo(data)
	if !found || err != nil {
		return nil, found, err
	}
	home, err := util.UserHomeDir()
	if err != nil {
		return nil, true, err
	}
	args, err := buildPWAOpenArgs(config, info, home, background)
	return args, true, err
}

func expandBrowserAppPath(identifier string) (string, error) {
	if !strings.HasPrefix(identifier, "~/") {
		return identifier, nil
	}
	home, err := util.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, strings.TrimPrefix(identifier, "~/")), nil
}

func parsePWAInfo(data []byte) (pwaInfo, bool, error) {
	if len(data) == 0 {
		return pwaInfo{}, false, nil
	}
	result := C.finickyPWAInfoJSON(unsafe.Pointer(&data[0]), C.size_t(len(data)))
	if result == nil {
		return pwaInfo{}, false, nil
	}
	defer C.free(unsafe.Pointer(result))
	var info pwaInfo
	if err := json.Unmarshal([]byte(C.GoString(result)), &info); err != nil {
		return info, true, fmt.Errorf("could not decode web app metadata: %w", err)
	}
	if info.Error != "" {
		return info, true, fmt.Errorf("%s", info.Error)
	}
	return info, true, nil
}

func buildPWAOpenArgs(config BrowserConfig, info pwaInfo, home string, background bool) ([]string, error) {
	if config.Private {
		return nil, fmt.Errorf("installed web apps cannot open in a private window; choose a browser instead")
	}
	entry := lookupBrowser(info.BrowserID)
	if entry == nil || entry.Type != "Chromium" || entry.ID != info.BrowserID || entry.ConfigDirRelative == "" {
		return nil, fmt.Errorf("web app's parent browser %q is not supported", info.BrowserID)
	}
	if !pwaIDPattern.MatchString(info.AppID) {
		return nil, fmt.Errorf("web app has an invalid application ID")
	}
	link, err := url.Parse(config.URL)
	if err != nil || (link.Scheme != "http" && link.Scheme != "https") || link.Hostname() == "" {
		return nil, fmt.Errorf("web apps require an http or https link")
	}
	profile := info.Profile
	if config.ProfileDirectory != "" {
		if profile != "" && profile != config.ProfileDirectory {
			return nil, fmt.Errorf("selected profile does not match the installed web app's profile")
		}
		profile = config.ProfileDirectory
	} else if config.Profile != "" {
		args, ok := resolveBrowserProfileArgs(info.BrowserID, config.Profile)
		if !ok || len(args) != 1 || !strings.HasPrefix(args[0], "--profile-directory=") {
			return nil, fmt.Errorf("could not resolve profile %q for web app", config.Profile)
		}
		resolved := strings.TrimPrefix(args[0], "--profile-directory=")
		if profile != "" && profile != resolved {
			return nil, fmt.Errorf("selected profile does not match the installed web app's profile")
		}
		profile = resolved
	}
	if profile == "" {
		return nil, fmt.Errorf("this web app is shared by several profiles; choose an explicit profile directory")
	}
	profileArgs, err := directProfileArgs(info.BrowserID, profile)
	if err != nil {
		return nil, err
	}
	// Chromium stores its app-data directory here, not the user-data root.
	// Multi-profile wrappers replace the profile component with "-".
	if info.DataDir != "" {
		dataDir := filepath.Clean(info.DataDir)
		webApps := filepath.Dir(dataDir)
		profileDir := filepath.Dir(webApps)
		root := filepath.Dir(profileDir)
		want := filepath.Join(home, "Library/Application Support", entry.ConfigDirRelative)
		if !filepath.IsAbs(info.DataDir) || filepath.Base(webApps) != "Web Applications" || root != want ||
			(filepath.Base(profileDir) != profile && filepath.Base(profileDir) != "-") {
			return nil, fmt.Errorf("web app uses a nonstandard browser data folder; select a browser profile instead")
		}
	}
	openArgs := []string{"-b", info.BrowserID}
	if config.OpenInBackground != nil {
		background = *config.OpenInBackground
	}
	if background {
		openArgs = append(openArgs, "-g")
	}
	custom := config.Args
	if separator := slices.Index(custom, "--args"); separator >= 0 {
		openArgs = append(openArgs, custom[:separator]...)
		custom = custom[separator+1:]
	}
	if !slices.Contains(openArgs, "-n") {
		openArgs = append(openArgs, "-n")
	}
	openArgs = append(openArgs, "--args")
	openArgs = append(openArgs, profileArgs...)
	if config.NewWindow {
		openArgs = append(openArgs, "--new-window")
	}
	for _, arg := range custom {
		// Launch URL is carried only by Chromium's app launch switch. A second
		// positional URL would also open an ordinary browser tab.
		if arg == config.URL {
			continue
		}
		key := strings.SplitN(arg, "=", 2)[0]
		switch key {
		case "--app", "--app-id", "--app-launch-url-for-shortcuts-menu-item", "--profile-directory", "--user-data-dir", "--incognito", "--inprivate", "--guest":
			return nil, fmt.Errorf("custom argument %q conflicts with the installed web app", key)
		}
		if !strings.HasPrefix(arg, "-") {
			return nil, fmt.Errorf("web app custom arguments must be flags, not additional URLs")
		}
		openArgs = append(openArgs, arg)
	}
	openArgs = append(openArgs, "--app-id="+info.AppID, "--app-launch-url-for-shortcuts-menu-item="+config.URL)
	return openArgs, nil
}
