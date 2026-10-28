package version

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"finicky/util"

	"github.com/Masterminds/semver"
	"github.com/dop251/goja"
)

const (
	updateCheckInterval    = 24 * time.Hour
	defaultGitHubReleases  = "https://api.github.com/repos/kevincorvallis/waystile/releases/latest"
	defaultReleasePageURL  = "https://github.com/kevincorvallis/waystile/releases"
	cloudManifestName      = "release.json"
	cloudUpdateArchiveName = "Waystile-Update.zip"
	cloudSetupArchiveName  = "Finicky Plus Setup.zip"
)

type ReleaseInfo struct {
	HasUpdate               bool   `json:"hasUpdate"`
	LatestVersion           string `json:"latestVersion"`
	LatestCommit            string `json:"latestCommit,omitempty"`
	PublishedAt             string `json:"publishedAt,omitempty"`
	DownloadUrl             string `json:"downloadUrl"`
	ReleaseUrl              string `json:"releaseUrl"`
	Source                  string `json:"source,omitempty"`
	CanInstallAutomatically bool   `json:"canInstallAutomatically,omitempty"`
	ArchivePendingDownload  bool   `json:"archivePendingDownload,omitempty"`
	IncludesCompanion       bool   `json:"includesCompanion,omitempty"`
	IncludesChromeExtension bool   `json:"includesChromeExtension,omitempty"`
	Notes                   string `json:"notes,omitempty"`
}

type UpdateCheckInfo struct {
	Timestamp   int64       `json:"timestamp"`
	ReleaseInfo ReleaseInfo `json:"releaseInfo"`
}

var (
	// These will be set via ldflags during build
	commitHash = "dev"
	buildDate  = "unknown"
	apiHost    = ""
)

var plistVersionKeyPattern = regexp.MustCompile(`(?s)<key>(?:CFBundleShortVersionString|CFBundleVersion)</key>\s*<string>\s*([^<\s]+)\s*</string>`)

// GetBuildInfo returns the commit hash and build date
func GetBuildInfo() (string, string) {
	return commitHash, buildDate
}

func readVersionFromPlistBytes(data []byte) string {
	matches := plistVersionKeyPattern.FindSubmatch(data)
	if len(matches) < 2 {
		return ""
	}
	return strings.TrimSpace(string(matches[1]))
}

func getCacheDir() string {
	cacheDir, err := util.UserCacheDir()
	if err != nil {
		slog.Error("Error getting user cache directory", "error", err)
		return ""
	}
	cacheDir = filepath.Join(cacheDir, "Finicky Plus")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		slog.Error("Error creating cache directory", "error", err)
		return ""
	}
	return cacheDir
}

func getLastUpdateCheck() *UpdateCheckInfo {
	cacheDir := getCacheDir()
	if cacheDir == "" {
		return nil
	}

	cacheFile := filepath.Join(cacheDir, "last_update_check.json")
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		return nil
	}

	var info UpdateCheckInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil
	}

	return &info
}

func setLastUpdateCheck(info UpdateCheckInfo) {
	cacheDir := getCacheDir()
	if cacheDir == "" {
		return
	}

	data, err := json.Marshal(info)
	if err != nil {
		slog.Error("Error marshaling update check info", "error", err)
		return
	}

	cacheFile := filepath.Join(cacheDir, "last_update_check.json")
	if err := os.WriteFile(cacheFile, data, 0644); err != nil {
		slog.Error("Error saving last update check info", "error", err)
	}
}

func GetCurrentVersion() string {
	candidates := []string{}
	if custom := os.Getenv("FINICKY_BUNDLE_PATH"); custom != "" {
		candidates = append(candidates, custom)
	}
	if execPath, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(execPath), "..", "Info.plist"))
	}
	if legacy := os.Getenv("BUNDLE_PATH"); legacy != "" {
		candidates = append(candidates, legacy)
	}

	for _, candidate := range candidates {
		plistPath := candidate
		if info, err := os.Stat(plistPath); err == nil && info.IsDir() {
			plistPath = filepath.Join(plistPath, "Contents", "Info.plist")
		}
		if data, err := os.ReadFile(plistPath); err == nil {
			if v := readVersionFromPlistBytes(data); v != "" {
				return v
			}
		}
	}

	return "dev"
}

func checkForUpdates(forceNetwork bool) (releaseInfo *ReleaseInfo) {
	currentVersion := GetCurrentVersion()
	if currentVersion == "" {
		currentVersion = "dev"
	}

	// 1. Always check the local iCloud Drive release manifest first (zero network cost).
	if cloudInfo := CheckCloudUpdate(DefaultCloudSetupDir(), currentVersion, commitHash, buildDate); cloudInfo != nil {
		return cloudInfo
	}

	if currentVersion == "dev" {
		return nil
	}

	// 2. Fall back to cached or fresh remote release check.
	updateCheckInfo := getLastUpdateCheck()
	if !forceNetwork && updateCheckInfo != nil && updateCheckInfo.Timestamp > 0 {
		timeSinceLastCheck := time.Since(time.Unix(updateCheckInfo.Timestamp, 0))
		if timeSinceLastCheck < updateCheckInterval {
			if updateCheckInfo.ReleaseInfo.LatestVersion == "" {
				return nil
			}
			updateAvailable, err := isUpdateAvailable(currentVersion, updateCheckInfo.ReleaseInfo.LatestVersion)
			if err != nil || !updateAvailable {
				return nil
			}
			cached := updateCheckInfo.ReleaseInfo
			cached.HasUpdate = true
			return &cached
		}
	}

	client := &http.Client{
		Timeout: 4 * time.Second,
	}

	if apiHost != "" {
		info := checkCustomAPIHost(client, currentVersion)
		if info != nil {
			return info
		}
	}

	return checkGitHubReleases(client, currentVersion)
}

func checkCustomAPIHost(client *http.Client, currentVersion string) *ReleaseInfo {
	apiUrl := fmt.Sprintf("%s/update-check?version=%s", apiHost, currentVersion)
	req, err := http.NewRequest("GET", apiUrl, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", fmt.Sprintf("waystile/%s", currentVersion))
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil
	}
	var releaseInfo ReleaseInfo
	if err := json.Unmarshal(body, &releaseInfo); err != nil || releaseInfo.LatestVersion == "" {
		return nil
	}
	releaseInfo.Source = "api"
	updateAvailable, err := isUpdateAvailable(currentVersion, releaseInfo.LatestVersion)
	if err != nil {
		return nil
	}
	releaseInfo.HasUpdate = updateAvailable
	setLastUpdateCheck(UpdateCheckInfo{
		Timestamp:   time.Now().Unix(),
		ReleaseInfo: releaseInfo,
	})
	if !updateAvailable {
		return nil
	}
	return &releaseInfo
}

type githubReleasePayload struct {
	TagName     string `json:"tag_name"`
	HTMLURL     string `json:"html_url"`
	PublishedAt string `json:"published_at"`
	Body        string `json:"body"`
	Draft       bool   `json:"draft"`
	Assets      []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func checkGitHubReleases(client *http.Client, currentVersion string) *ReleaseInfo {
	endpoint := os.Getenv("FINICKY_GITHUB_RELEASES_URL")
	if endpoint == "" {
		endpoint = defaultGitHubReleases
	}
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", fmt.Sprintf("waystile/%s", currentVersion))
	resp, err := client.Do(req)
	if err != nil {
		slog.Debug("GitHub update check skipped or unreachable", "error", err)
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil
	}
	var payload githubReleasePayload
	if err := json.Unmarshal(body, &payload); err != nil || payload.Draft || payload.TagName == "" {
		return nil
	}
	releaseURL := payload.HTMLURL
	if releaseURL == "" {
		releaseURL = defaultReleasePageURL
	}
	downloadURL := releaseURL
	for _, asset := range payload.Assets {
		lower := strings.ToLower(asset.Name)
		if strings.HasSuffix(lower, ".zip") && (strings.Contains(lower, "waystile") || strings.Contains(lower, "universal")) && !strings.Contains(lower, "actions") {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}
	updateAvailable, err := isUpdateAvailable(currentVersion, payload.TagName)
	if err != nil {
		return nil
	}
	info := ReleaseInfo{
		HasUpdate:     updateAvailable,
		LatestVersion: strings.TrimPrefix(payload.TagName, "v"),
		PublishedAt:   payload.PublishedAt,
		DownloadUrl:   downloadURL,
		ReleaseUrl:    releaseURL,
		Source:        "github",
		Notes:         strings.TrimSpace(payload.Body),
	}
	setLastUpdateCheck(UpdateCheckInfo{
		Timestamp:   time.Now().Unix(),
		ReleaseInfo: info,
	})
	if !updateAvailable {
		return nil
	}
	return &info
}

// CheckForUpdatesIfEnabled checks if updates should be performed based on VM configuration
func CheckForUpdatesIfEnabled(vm *goja.Runtime) (releaseInfo *ReleaseInfo, updateCheckEnabled bool, err error) {
	return CheckForUpdatesWithOptions(vm, false)
}

// CheckForUpdatesWithOptions checks for updates, optionally forcing a fresh check.
func CheckForUpdatesWithOptions(vm *goja.Runtime, force bool) (releaseInfo *ReleaseInfo, updateCheckEnabled bool, err error) {
	if mockVersion := os.Getenv("FINICKY_MOCK_UPDATE"); mockVersion != "" {
		slog.Info("FINICKY_MOCK_UPDATE set, returning mock update", "version", mockVersion)
		mockTag := strings.TrimPrefix(mockVersion, "v")
		return &ReleaseInfo{
			HasUpdate:     true,
			LatestVersion: mockVersion,
			DownloadUrl:   fmt.Sprintf("https://github.com/kevincorvallis/waystile/releases/tag/v%s", mockTag),
			ReleaseUrl:    fmt.Sprintf("https://github.com/kevincorvallis/waystile/releases/tag/v%s", mockTag),
			Source:        "github",
		}, true, nil
	}

	// Always check local iCloud Drive release manifest when force-checking or when enabled.
	if vm == nil || force {
		releaseInfo := checkForUpdates(force)
		return releaseInfo, true, nil
	}

	shouldCheckForUpdates, err := vm.RunString("finickyConfigAPI.getOption('checkForUpdates', finalConfig, true)")
	if err != nil {
		return nil, true, fmt.Errorf("failed to get checkForUpdates option: %v", err)
	}

	// Even if remote HTTP update checks are disabled in old configs, local iCloud Drive
	// release manifests are checked with zero network traffic.
	if !shouldCheckForUpdates.ToBoolean() {
		if cloudInfo := CheckCloudUpdate(DefaultCloudSetupDir(), GetCurrentVersion(), commitHash, buildDate); cloudInfo != nil {
			return cloudInfo, false, nil
		}
		return nil, false, nil
	}

	return checkForUpdates(false), true, nil
}

// isUpdateAvailable checks if the latest version is newer than the current version
func isUpdateAvailable(currentVersion, latestVersion string) (bool, error) {
	currentVersion = strings.TrimPrefix(currentVersion, "v")
	latestVersion = strings.TrimPrefix(latestVersion, "v")

	currentSemver, err := semver.NewVersion(currentVersion)
	if err != nil {
		return false, fmt.Errorf("parsing current version %s: %w", currentVersion, err)
	}

	latestSemver, err := semver.NewVersion(latestVersion)
	if err != nil {
		return false, fmt.Errorf("parsing latest version %s: %w", latestVersion, err)
	}

	return latestSemver.GreaterThan(currentSemver), nil
}
