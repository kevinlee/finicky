package browser

import (
	"finicky/util"
	"path/filepath"
	"sort"
)

type BrowserProfile struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Profile          string `json:"profile,omitempty"`
	ProfileDirectory string `json:"profileDirectory,omitempty"`
}

// GetBrowserProfiles retains actual Chromium directory IDs so a renamed or
// duplicate profile can never silently open a different account.
func GetBrowserProfiles(identifier string) []BrowserProfile {
	entry := lookupBrowser(identifier)
	if entry == nil {
		return []BrowserProfile{}
	}
	home, err := util.UserHomeDir()
	if err != nil {
		return []BrowserProfile{}
	}
	base := filepath.Join(home, "Library/Application Support", entry.ConfigDirRelative)
	return readBrowserProfiles(*entry, base)
}

func readBrowserProfiles(entry browserInfo, base string) []BrowserProfile {
	profiles := []BrowserProfile{}
	switch entry.Type {
	case "Chromium":
		cache, ok := chromiumInfoCache(filepath.Join(base, "Local State"))
		if !ok {
			return profiles
		}
		for directory, raw := range cache {
			info, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			name, _ := info["name"].(string)
			if name == "" {
				name = directory
			}
			if _, err := directProfileArgs(entry.ID, directory); err != nil {
				continue
			}
			profiles = append(profiles, BrowserProfile{ID: entry.ID + ":" + directory, Name: name, Profile: name, ProfileDirectory: directory})
		}
	case "Firefox":
		for _, record := range readFirefoxProfileRecords(base) {
			profile := record.name
			if record.modern {
				profile = record.path
			}
			profiles = append(profiles, BrowserProfile{ID: entry.ID + ":" + record.path, Name: record.name, Profile: profile})
		}

	}
	sort.Slice(profiles, func(i, j int) bool {
		if profiles[i].Name == profiles[j].Name {
			return profiles[i].ID < profiles[j].ID
		}
		return profiles[i].Name < profiles[j].Name
	})
	return profiles
}
