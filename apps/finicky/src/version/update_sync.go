package version

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

const maxUpdateArchiveBytes = 200 * 1024 * 1024

var (
	sha256HexPattern  = regexp.MustCompile(`^[a-f0-9]{64}$`)
	commitHexPattern  = regexp.MustCompile(`^[a-f0-9]{7,40}$`)
	teamIDPattern     = regexp.MustCompile(`^[A-Z0-9]{10}$`)
	plistBundleIDExpr = regexp.MustCompile(`(?s)<key>CFBundleIdentifier</key>\s*<string>\s*([^<\s]+)\s*</string>`)
	plistExecNameExpr = regexp.MustCompile(`(?s)<key>CFBundleExecutable</key>\s*<string>\s*([^<]+?)\s*</string>`)
)

type CloudReleaseManifest struct {
	SchemaVersion           int    `json:"schemaVersion"`
	Version                 string `json:"version"`
	CommitHash              string `json:"commitHash"`
	BuildDate               string `json:"buildDate"`
	PublishedAt             string `json:"publishedAt"`
	ArchiveName             string `json:"archiveName"`
	SHA256                  string `json:"sha256"`
	Size                    int64  `json:"size"`
	TeamID                  string `json:"teamId,omitempty"`
	IncludesCompanion       bool   `json:"includesCompanion"`
	IncludesChromeExtension bool   `json:"includesChromeExtension"`
	Notes                   string `json:"notes,omitempty"`
}

type PublishOptions struct {
	CloudSetupDir      string
	RouterAppPath      string
	CompanionAppPath   string
	ChromeExtensionDir string
	Notes              string
	AllowAdHocForTest  bool
	AllowSingleArch    bool
}

type InstallOptions struct {
	CloudSetupDir      string
	InstallDir         string
	BackupRootDir      string
	ChromeExtensionDir string
	RequiredTeamID     string
	AllowAdHocForTest  bool
	AllowSingleArch    bool
	SkipLSRegister     bool
}

type InstallResult struct {
	Version                 string `json:"version"`
	CommitHash              string `json:"commitHash"`
	InstalledAppPath        string `json:"installedAppPath"`
	InstalledCompanionPath  string `json:"installedCompanionPath,omitempty"`
	UpdatedChromeExtension  bool   `json:"updatedChromeExtension"`
	BackupDir               string `json:"backupDir,omitempty"`
}

var (
	downloadCooldownMu sync.Mutex
	downloadRequested  = map[string]time.Time{}
)

// DefaultCloudSetupDir returns the iCloud Drive folder used to stage Waystile builds across Macs.
func DefaultCloudSetupDir() string {
	if custom := strings.TrimSpace(os.Getenv("FINICKY_CLOUD_UPDATE_DIR")); custom != "" {
		return custom
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, "Library", "Mobile Documents", "com~apple~CloudDocs", "Finicky Plus Setup")
}

func ValidateCloudManifest(m CloudReleaseManifest) error {
	if m.SchemaVersion != 1 {
		return fmt.Errorf("unsupported update manifest schema version %d", m.SchemaVersion)
	}
	if strings.TrimSpace(m.Version) == "" {
		return fmt.Errorf("update manifest is missing version")
	}
	if _, err := isUpdateAvailable("0.0.1", m.Version); err != nil {
		return fmt.Errorf("invalid manifest version %q: %w", m.Version, err)
	}
	if m.CommitHash != "" && m.CommitHash != "dev" && !commitHexPattern.MatchString(m.CommitHash) {
		return fmt.Errorf("invalid manifest commit hash %q", m.CommitHash)
	}
	if m.ArchiveName == "" || filepath.Base(m.ArchiveName) != m.ArchiveName || strings.ContainsAny(m.ArchiveName, `/\`) || !strings.HasSuffix(strings.ToLower(m.ArchiveName), ".zip") {
		return fmt.Errorf("invalid update archive name %q", m.ArchiveName)
	}
	if !sha256HexPattern.MatchString(strings.ToLower(m.SHA256)) {
		return fmt.Errorf("invalid update archive SHA-256")
	}
	if m.Size <= 0 || m.Size > maxUpdateArchiveBytes {
		return fmt.Errorf("update archive size %d is out of bounds", m.Size)
	}
	if m.TeamID != "" && !teamIDPattern.MatchString(m.TeamID) {
		return fmt.Errorf("invalid signing team ID %q", m.TeamID)
	}
	return nil
}

func placeholderPath(targetPath string) string {
	return filepath.Join(filepath.Dir(targetPath), "."+filepath.Base(targetPath)+".icloud")
}

func requestICloudDownload(targetPath string) {
	if targetPath == "" {
		return
	}
	downloadCooldownMu.Lock()
	last, ok := downloadRequested[targetPath]
	if ok && time.Since(last) < 30*time.Second {
		downloadCooldownMu.Unlock()
		return
	}
	downloadRequested[targetPath] = time.Now()
	downloadCooldownMu.Unlock()

	if _, err := exec.LookPath("brctl"); err == nil {
		_ = exec.Command("brctl", "download", targetPath).Start()
	}
}

func ReadCloudManifest(cloudSetupDir string) (*CloudReleaseManifest, bool, error) {
	if cloudSetupDir == "" {
		return nil, false, errors.New("cloud setup directory is not configured")
	}
	manifestPath := filepath.Join(cloudSetupDir, cloudManifestName)
	if _, err := os.Stat(placeholderPath(manifestPath)); err == nil {
		requestICloudDownload(manifestPath)
		return nil, true, nil
	}
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var m CloudReleaseManifest
	if err := decoder.Decode(&m); err != nil {
		return nil, false, fmt.Errorf("malformed cloud update manifest: %w", err)
	}
	if err := ValidateCloudManifest(m); err != nil {
		return nil, false, err
	}
	archivePath := filepath.Join(cloudSetupDir, m.ArchiveName)
	archivePending := false
	if _, err := os.Stat(placeholderPath(archivePath)); err == nil {
		archivePending = true
		requestICloudDownload(archivePath)
	} else if info, err := os.Stat(archivePath); err != nil || info.Size() != m.Size {
		archivePending = true
		requestICloudDownload(archivePath)
	}
	return &m, archivePending, nil
}

func parseBuildTimestamp(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" || value == "unknown" {
		return time.Time{}, false
	}
	for _, layout := range []string{
		"2006-01-02 15:04:05 MST",
		time.RFC3339,
		time.RFC3339Nano,
	} {
		if t, err := time.Parse(layout, value); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}

// IsCloudUpdateNewer determines whether a cloud release manifest is newer than the running binary.
// It checks semantic version first, then falls back to build timestamp + commit hash when
// the semantic version is identical so same-version builds published across Macs are detected.
func IsCloudUpdateNewer(currentVersion, currentCommit, currentBuildDate string, m CloudReleaseManifest) bool {
	if currentVersion == "" || currentVersion == "dev" {
		return false
	}
	newerSemver, err := isUpdateAvailable(currentVersion, m.Version)
	if err != nil {
		return false
	}
	if newerSemver {
		return true
	}
	olderSemver, err := isUpdateAvailable(m.Version, currentVersion)
	if err != nil || olderSemver {
		return false
	}
	// Same semantic version: check if it is a newer build commit.
	if m.CommitHash == "" || m.CommitHash == "dev" || currentCommit == "" || currentCommit == "dev" {
		return false
	}
	if strings.EqualFold(m.CommitHash, currentCommit) {
		return false
	}
	manifestTime, okManifest := parseBuildTimestamp(m.BuildDate)
	if !okManifest {
		manifestTime, okManifest = parseBuildTimestamp(m.PublishedAt)
	}
	currentTime, okCurrent := parseBuildTimestamp(currentBuildDate)
	if okManifest && okCurrent {
		return manifestTime.After(currentTime)
	}
	return okManifest && !okCurrent
}

func CheckCloudUpdate(cloudSetupDir, currentVersion, currentCommit, currentBuildDate string) *ReleaseInfo {
	manifest, archivePending, err := ReadCloudManifest(cloudSetupDir)
	if err != nil || manifest == nil {
		return nil
	}
	hasUpdate := IsCloudUpdateNewer(currentVersion, currentCommit, currentBuildDate, *manifest)
	archivePath := filepath.Join(cloudSetupDir, manifest.ArchiveName)
	return &ReleaseInfo{
		HasUpdate:               hasUpdate,
		LatestVersion:           manifest.Version,
		LatestCommit:            manifest.CommitHash,
		PublishedAt:             manifest.PublishedAt,
		DownloadUrl:             "file://" + filepath.ToSlash(archivePath),
		ReleaseUrl:              defaultReleasePageURL,
		Source:                  "icloud",
		CanInstallAutomatically: true,
		ArchivePendingDownload:  archivePending,
		IncludesCompanion:       manifest.IncludesCompanion,
		IncludesChromeExtension: manifest.IncludesChromeExtension,
		Notes:                   manifest.Notes,
	}
}

func readPlistField(plistPath string, expr *regexp.Regexp) string {
	data, err := os.ReadFile(plistPath)
	if err != nil {
		return ""
	}
	matches := expr.FindSubmatch(data)
	if len(matches) < 2 {
		return ""
	}
	return strings.TrimSpace(string(matches[1]))
}

func BundleTeamID(bundlePath string) (string, error) {
	cmd := exec.Command("codesign", "-dv", "--verbose=4", bundlePath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("codesign inspection failed for %s: %w", filepath.Base(bundlePath), err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "TeamIdentifier=") {
			team := strings.TrimSpace(strings.TrimPrefix(line, "TeamIdentifier="))
			if team == "not set" {
				return "", nil
			}
			return team, nil
		}
	}
	return "", nil
}

func verifyBundleSignatureAndArch(bundlePath, expectedBundleID string, allowAdHoc, allowSingleArch bool) (string, error) {
	plistPath := filepath.Join(bundlePath, "Contents", "Info.plist")
	actualID := readPlistField(plistPath, plistBundleIDExpr)
	if actualID != expectedBundleID {
		return "", fmt.Errorf("unexpected bundle identifier %q for %s (expected %s)", actualID, filepath.Base(bundlePath), expectedBundleID)
	}
	execName := readPlistField(plistPath, plistExecNameExpr)
	if execName == "" {
		return "", fmt.Errorf("missing CFBundleExecutable in %s", filepath.Base(bundlePath))
	}
	execPath := filepath.Join(bundlePath, "Contents", "MacOS", execName)
	if !allowSingleArch {
		out, err := exec.Command("lipo", "-archs", execPath).Output()
		if err != nil {
			return "", fmt.Errorf("failed to inspect architectures for %s: %w", filepath.Base(bundlePath), err)
		}
		arches := " " + strings.TrimSpace(string(out)) + " "
		if !strings.Contains(arches, " arm64 ") || !strings.Contains(arches, " x86_64 ") {
			return "", fmt.Errorf("%s must be a universal binary (arm64 + x86_64), got %q", filepath.Base(bundlePath), strings.TrimSpace(string(out)))
		}
	}
	if out, err := exec.Command("codesign", "--verify", "--deep", "--strict", bundlePath).CombinedOutput(); err != nil {
		return "", fmt.Errorf("signature verification failed for %s: %s", filepath.Base(bundlePath), strings.TrimSpace(string(out)))
	}
	teamID, err := BundleTeamID(bundlePath)
	if err != nil {
		return "", err
	}
	if !allowAdHoc && teamID == "" {
		return "", fmt.Errorf("%s is ad-hoc signed; an Apple signing Team ID is required", filepath.Base(bundlePath))
	}
	return teamID, nil
}

func verifyCompanionBundle(bundlePath, expectedTeamID string, allowAdHoc, allowSingleArch bool) error {
	teamID, err := verifyBundleSignatureAndArch(bundlePath, "com.kevinlee.finicky.actions", allowAdHoc, allowSingleArch)
	if err != nil {
		return err
	}
	if expectedTeamID != "" && teamID != expectedTeamID {
		return fmt.Errorf("companion signing Team ID %q does not match router Team ID %q", teamID, expectedTeamID)
	}
	extensions := []struct {
		relPath  string
		bundleID string
	}{
		{"Contents/PlugIns/Finicky Safari.appex", "com.kevinlee.finicky.actions.safari"},
		{"Contents/PlugIns/Open with Finicky Plus.appex", "com.kevinlee.finicky.actions.share"},
		{"Contents/Extensions/Finicky Focus.appex", "com.kevinlee.finicky.actions.focus"},
	}
	for _, ext := range extensions {
		extPath := filepath.Join(bundlePath, ext.relPath)
		extTeam, err := verifyBundleSignatureAndArch(extPath, ext.bundleID, allowAdHoc, allowSingleArch)
		if err != nil {
			return err
		}
		if extTeam != teamID {
			return fmt.Errorf("embedded extension %s Team ID %q does not match %q", filepath.Base(extPath), extTeam, teamID)
		}
	}
	return nil
}

func sha256File(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

func atomicCopyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	dir := filepath.Dir(dst)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".waystile-stage-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := io.Copy(tmp, in); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, dst)
}

func atomicWriteBytes(dst string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(dst)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".waystile-write-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, dst)
}

func setupGuideMarkdown() string {
	return `# Set up or update Waystile on your other Mac

Use the same Apple Account with iCloud Drive enabled. Waystile shares visual routing rules and profile labels through iCloud Drive, and can also update the app across your Macs in one click.

## First-time install on another Mac

1. In Finder, open **iCloud Drive → Finicky Plus Setup** and let **Waystile-Update.zip** (or **Finicky Plus Setup.zip**) finish downloading. Unzip it in a local folder.
2. Quit any existing Waystile or Finicky Plus router on that Mac. Move **Waystile.app** and **Waystile Actions.app** into **/Applications** and open **Waystile.app**.
3. Open Waystile settings and choose **Sync**. Turn on **Sync with iCloud Drive**.
4. Map **Work**, **Personal**, and **Academic** to their local browser profile folders on this Mac, then click **Save profile mappings**.
5. Wait until the status says **Up to date**, then use **Test a link** to verify your routes. Set **Waystile** as the default web browser in macOS System Settings.

## One-click updates across Macs

Once Waystile is installed on both Macs:
- When you build or install an update on one Mac, click **Publish this Mac's build to iCloud** in **Sync** or **About** (or run ` + "`./scripts/publish-update.sh`" + `).
- Your other Mac will automatically detect the new release in iCloud Drive and display an **Update & Relaunch** banner.
- Clicking **Update & Relaunch** verifies the SHA-256 checksum and Apple code-signing Team ID, backs up the previous app in ` + "`~/Library/Application Support/Finicky Plus/backups/`" + `, updates **Waystile.app**, **Waystile Actions.app**, and the unpacked **Chrome Extension**, and restarts Waystile.
`
}

// PublishCloudUpdate packages the verified Waystile router, companion, and Chrome extension
// into the shared iCloud Drive setup folder and writes a signed-metadata release.json manifest.
func PublishCloudUpdate(opts PublishOptions) (*CloudReleaseManifest, error) {
	cloudDir := opts.CloudSetupDir
	if cloudDir == "" {
		cloudDir = DefaultCloudSetupDir()
	}
	if cloudDir == "" {
		return nil, errors.New("could not resolve iCloud Drive setup directory")
	}

	routerPath := opts.RouterAppPath
	if routerPath == "" {
		routerPath = "/Applications/Waystile.app"
	}
	if _, err := os.Stat(routerPath); err != nil {
		return nil, fmt.Errorf("router app not found at %s: %w", routerPath, err)
	}

	companionPath := opts.CompanionAppPath
	if companionPath == "" {
		candidate := filepath.Join(filepath.Dir(routerPath), "Waystile Actions.app")
		if _, err := os.Stat(candidate); err == nil {
			companionPath = candidate
		}
	}

	chromeExtDir := opts.ChromeExtensionDir
	if chromeExtDir == "" {
		if home, err := os.UserHomeDir(); err == nil {
			candidate := filepath.Join(home, "Library", "Application Support", "Finicky Plus", "Chrome Extension")
			if _, err := os.Stat(filepath.Join(candidate, "manifest.json")); err == nil {
				chromeExtDir = candidate
			}
		}
	}

	teamID, err := verifyBundleSignatureAndArch(routerPath, "com.kevinlee.finicky", opts.AllowAdHocForTest, opts.AllowSingleArch)
	if err != nil {
		return nil, err
	}

	includesCompanion := false
	if companionPath != "" {
		if _, statErr := os.Stat(companionPath); statErr == nil {
			if err := verifyCompanionBundle(companionPath, teamID, opts.AllowAdHocForTest, opts.AllowSingleArch); err != nil {
				return nil, err
			}
			includesCompanion = true
		}
	}

	plistBytes, err := os.ReadFile(filepath.Join(routerPath, "Contents", "Info.plist"))
	if err != nil {
		return nil, err
	}
	bundleVersion := readVersionFromPlistBytes(plistBytes)
	if bundleVersion == "" {
		return nil, errors.New("could not read CFBundleShortVersionString from router Info.plist")
	}

	cHash, bDate := GetBuildInfo()

	stageParent, err := os.MkdirTemp("", "waystile-publish-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(stageParent)
	if err := os.Chmod(stageParent, 0700); err != nil {
		return nil, err
	}

	stagePayload := filepath.Join(stageParent, "Waystile Setup")
	if err := os.MkdirAll(stagePayload, 0700); err != nil {
		return nil, err
	}

	if out, err := exec.Command("ditto", routerPath, filepath.Join(stagePayload, "Waystile.app")).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("failed to stage Waystile.app: %s", strings.TrimSpace(string(out)))
	}
	if includesCompanion {
		if out, err := exec.Command("ditto", companionPath, filepath.Join(stagePayload, "Waystile Actions.app")).CombinedOutput(); err != nil {
			return nil, fmt.Errorf("failed to stage Waystile Actions.app: %s", strings.TrimSpace(string(out)))
		}
	}
	includesChromeExt := false
	if chromeExtDir != "" {
		if _, err := os.Stat(filepath.Join(chromeExtDir, "manifest.json")); err == nil {
			if out, err := exec.Command("ditto", chromeExtDir, filepath.Join(stagePayload, "Chrome Extension")).CombinedOutput(); err != nil {
				return nil, fmt.Errorf("failed to stage Chrome Extension: %s", strings.TrimSpace(string(out)))
			}
			includesChromeExt = true
		}
	}

	guideText := []byte(setupGuideMarkdown())
	if err := os.WriteFile(filepath.Join(stagePayload, "Start Here.md"), guideText, 0644); err != nil {
		return nil, err
	}

	zipPath := filepath.Join(stageParent, cloudUpdateArchiveName)
	if out, err := exec.Command("ditto", "-c", "-k", "--sequesterRsrc", stagePayload, zipPath).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("failed to create update archive: %s", strings.TrimSpace(string(out)))
	}

	hash, size, err := sha256File(zipPath)
	if err != nil {
		return nil, err
	}

	notes := strings.TrimSpace(opts.Notes)
	if notes == "" {
		if cHash != "" && cHash != "dev" {
			notes = fmt.Sprintf("Waystile %s (%s)", bundleVersion, cHash)
		} else {
			notes = fmt.Sprintf("Waystile %s", bundleVersion)
		}
	}

	manifest := CloudReleaseManifest{
		SchemaVersion:           1,
		Version:                 bundleVersion,
		CommitHash:              cHash,
		BuildDate:               bDate,
		PublishedAt:             time.Now().UTC().Format(time.RFC3339),
		ArchiveName:             cloudUpdateArchiveName,
		SHA256:                  hash,
		Size:                    size,
		TeamID:                  teamID,
		IncludesCompanion:       includesCompanion,
		IncludesChromeExtension: includesChromeExt,
		Notes:                   notes,
	}
	if err := ValidateCloudManifest(manifest); err != nil {
		return nil, err
	}

	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(cloudDir, 0755); err != nil {
		return nil, err
	}
	if err := atomicCopyFile(zipPath, filepath.Join(cloudDir, cloudUpdateArchiveName), 0644); err != nil {
		return nil, err
	}
	// Keep Finicky Plus Setup.zip and Start Here.md updated for second-Mac initial setup compatibility.
	if err := atomicCopyFile(zipPath, filepath.Join(cloudDir, cloudSetupArchiveName), 0644); err != nil {
		return nil, err
	}
	if err := atomicWriteBytes(filepath.Join(cloudDir, "Start Here.md"), guideText, 0644); err != nil {
		return nil, err
	}
	if err := atomicWriteBytes(filepath.Join(cloudDir, cloudManifestName), manifestBytes, 0644); err != nil {
		return nil, err
	}

	return &manifest, nil
}

// InstallCloudUpdate verifies and installs the staged update from iCloud Drive.
func InstallCloudUpdate(opts InstallOptions) (*InstallResult, error) {
	cloudDir := opts.CloudSetupDir
	if cloudDir == "" {
		cloudDir = DefaultCloudSetupDir()
	}
	if cloudDir == "" {
		return nil, errors.New("could not resolve iCloud Drive setup directory")
	}

	manifest, archivePending, err := ReadCloudManifest(cloudDir)
	if err != nil {
		return nil, err
	}
	if manifest == nil {
		if archivePending {
			return nil, errors.New("waiting for iCloud Drive to download release metadata; try again in a moment")
		}
		return nil, errors.New("no published update manifest found in iCloud Drive")
	}
	if archivePending {
		return nil, errors.New("waiting for iCloud Drive to finish downloading the update archive; try again in a moment")
	}

	archivePath := filepath.Join(cloudDir, manifest.ArchiveName)
	actualHash, actualSize, err := sha256File(archivePath)
	if err != nil {
		return nil, fmt.Errorf("could not read update archive: %w", err)
	}
	if actualSize != manifest.Size {
		return nil, fmt.Errorf("update archive size mismatch (expected %d bytes, got %d)", manifest.Size, actualSize)
	}
	if !strings.EqualFold(actualHash, manifest.SHA256) {
		return nil, errors.New("update archive SHA-256 verification failed")
	}

	installDir := opts.InstallDir
	if installDir == "" {
		installDir = "/Applications"
	}

	requiredTeam := opts.RequiredTeamID
	if requiredTeam == "" && !opts.AllowAdHocForTest {
		if execPath, err := os.Executable(); err == nil {
			appBundle := filepath.Clean(filepath.Join(filepath.Dir(execPath), "..", ".."))
			if strings.HasSuffix(appBundle, ".app") {
				if t, err := BundleTeamID(appBundle); err == nil && t != "" {
					requiredTeam = t
				}
			}
		}
	}
	if requiredTeam != "" && manifest.TeamID != "" && requiredTeam != manifest.TeamID {
		return nil, fmt.Errorf("update manifest signing Team ID %q does not match installed Team ID %q", manifest.TeamID, requiredTeam)
	}

	extractDir, err := os.MkdirTemp("", "waystile-install-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(extractDir)
	if err := os.Chmod(extractDir, 0700); err != nil {
		return nil, err
	}

	if out, err := exec.Command("ditto", "-x", "-k", archivePath, extractDir).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("failed to unpack update archive: %s", strings.TrimSpace(string(out)))
	}

	stagedRouter := filepath.Join(extractDir, "Waystile.app")
	if _, err := os.Stat(stagedRouter); err != nil {
		return nil, errors.New("update archive does not contain Waystile.app")
	}
	routerTeam, err := verifyBundleSignatureAndArch(stagedRouter, "com.kevinlee.finicky", opts.AllowAdHocForTest, opts.AllowSingleArch)
	if err != nil {
		return nil, err
	}
	if requiredTeam != "" && routerTeam != requiredTeam {
		return nil, fmt.Errorf("incoming Waystile.app Team ID %q does not match required Team ID %q", routerTeam, requiredTeam)
	}
	if manifest.TeamID != "" && routerTeam != manifest.TeamID {
		return nil, fmt.Errorf("incoming Waystile.app Team ID %q does not match manifest Team ID %q", routerTeam, manifest.TeamID)
	}

	stagedCompanion := filepath.Join(extractDir, "Waystile Actions.app")
	hasCompanion := false
	if _, err := os.Stat(stagedCompanion); err == nil {
		if err := verifyCompanionBundle(stagedCompanion, routerTeam, opts.AllowAdHocForTest, opts.AllowSingleArch); err != nil {
			return nil, err
		}
		hasCompanion = true
	}

	backupRoot := opts.BackupRootDir
	if backupRoot == "" {
		if home, err := os.UserHomeDir(); err == nil {
			backupRoot = filepath.Join(home, "Library", "Application Support", "Finicky Plus", "backups")
		}
	}
	var backupDir string
	if backupRoot != "" {
		backupDir = filepath.Join(backupRoot, fmt.Sprintf("update-%s", time.Now().UTC().Format("20060102-150405")))
		if err := os.MkdirAll(backupDir, 0700); err != nil {
			return nil, err
		}
		for _, legacyOrCurrent := range []string{"Waystile.app", "Finicky Plus.app", "Waystile Actions.app", "Finicky Actions.app"} {
			existing := filepath.Join(installDir, legacyOrCurrent)
			if _, err := os.Stat(existing); err == nil {
				_ = exec.Command("ditto", existing, filepath.Join(backupDir, legacyOrCurrent)).Run()
			}
		}
	}

	targetRouter := filepath.Join(installDir, "Waystile.app")
	tmpRouter := filepath.Join(installDir, ".Waystile.app.updating")
	_ = os.RemoveAll(tmpRouter)
	if out, err := exec.Command("ditto", stagedRouter, tmpRouter).CombinedOutput(); err != nil {
		_ = os.RemoveAll(tmpRouter)
		return nil, fmt.Errorf("failed to copy Waystile.app to %s: %s", installDir, strings.TrimSpace(string(out)))
	}
	_ = os.RemoveAll(targetRouter)
	if err := os.Rename(tmpRouter, targetRouter); err != nil {
		_ = os.RemoveAll(tmpRouter)
		return nil, fmt.Errorf("failed to activate updated Waystile.app: %w", err)
	}
	// If migrating from Finicky Plus.app in the same directory, remove the old bundle after backup so LaunchServices does not route to it.
	legacyRouter := filepath.Join(installDir, "Finicky Plus.app")
	if _, err := os.Stat(legacyRouter); err == nil && backupDir != "" {
		_ = os.RemoveAll(legacyRouter)
	}

	var installedCompanion string
	if hasCompanion {
		targetCompanion := filepath.Join(installDir, "Waystile Actions.app")
		tmpCompanion := filepath.Join(installDir, ".Waystile Actions.app.updating")
		_ = os.RemoveAll(tmpCompanion)
		if out, err := exec.Command("ditto", stagedCompanion, tmpCompanion).CombinedOutput(); err != nil {
			_ = os.RemoveAll(tmpCompanion)
			return nil, fmt.Errorf("failed to copy Waystile Actions.app: %s", strings.TrimSpace(string(out)))
		}
		_ = os.RemoveAll(targetCompanion)
		if err := os.Rename(tmpCompanion, targetCompanion); err != nil {
			_ = os.RemoveAll(tmpCompanion)
			return nil, fmt.Errorf("failed to activate updated Waystile Actions.app: %w", err)
		}
		installedCompanion = targetCompanion
		legacyCompanion := filepath.Join(installDir, "Finicky Actions.app")
		if _, err := os.Stat(legacyCompanion); err == nil && backupDir != "" {
			_ = os.RemoveAll(legacyCompanion)
		}
	}

	chromeExtDir := opts.ChromeExtensionDir
	if chromeExtDir == "" {
		if home, err := os.UserHomeDir(); err == nil {
			chromeExtDir = filepath.Join(home, "Library", "Application Support", "Finicky Plus", "Chrome Extension")
		}
	}
	updatedChrome := false
	stagedChrome := filepath.Join(extractDir, "Chrome Extension")
	if chromeExtDir != "" {
		if _, err := os.Stat(filepath.Join(stagedChrome, "manifest.json")); err == nil {
			if err := os.MkdirAll(chromeExtDir, 0755); err == nil {
				if exec.Command("ditto", stagedChrome, chromeExtDir).Run() == nil {
					updatedChrome = true
				}
			}
		}
	}

	if !opts.SkipLSRegister {
		lsregister := "/System/Library/Frameworks/CoreServices.framework/Versions/Current/Frameworks/LaunchServices.framework/Versions/Current/Support/lsregister"
		if _, err := os.Stat(lsregister); err == nil {
			if backupDir != "" {
				for _, b := range []string{"Waystile.app", "Finicky Plus.app", "Waystile Actions.app", "Finicky Actions.app"} {
					p := filepath.Join(backupDir, b)
					if _, err := os.Stat(p); err == nil {
						_ = exec.Command(lsregister, "-u", p).Run()
					}
				}
			}
			_ = exec.Command(lsregister, "-f", "-R", "-trusted", targetRouter).Run()
			if installedCompanion != "" {
				_ = exec.Command(lsregister, "-f", "-R", "-trusted", installedCompanion).Run()
			}
		}
	}

	return &InstallResult{
		Version:                manifest.Version,
		CommitHash:             manifest.CommitHash,
		InstalledAppPath:       targetRouter,
		InstalledCompanionPath: installedCompanion,
		UpdatedChromeExtension: updatedChrome,
		BackupDir:              backupDir,
	}, nil
}

// RelaunchInstalledApp spawns a detached shell that waits briefly, launches the newly
// installed Waystile.app with the settings window open, and exits the current process.
func RelaunchInstalledApp(appPath string) error {
	if appPath == "" {
		appPath = "/Applications/Waystile.app"
	}
	cmd := exec.Command("/bin/sh", "-c", `sleep 0.6 && open -n "$1" --args --window`, "waystile-relaunch", appPath)
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() {
		time.Sleep(150 * time.Millisecond)
		os.Exit(0)
	}()
	return nil
}
