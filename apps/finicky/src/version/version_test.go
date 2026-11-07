package version

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestIsUpdateAvailable(t *testing.T) {
	tests := []struct {
		name     string
		current  string
		latest   string
		expected bool
		wantErr  bool
	}{
		{"same version", "1.0.0", "1.0.0", false, false},
		{"newer version available", "1.0.0", "1.0.1", true, false},
		{"older version", "1.0.1", "1.0.0", false, false},
		{"v-prefixed versions", "v1.0.0", "v1.0.1", true, false},
		{"mixed prefixes", "1.0.0", "v1.0.1", true, false},
		{"pre-release vs stable", "1.0.0-beta", "1.0.0", true, false},
		{"invalid current version", "invalid", "1.0.0", false, true},
		{"invalid latest version", "1.0.0", "invalid", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := isUpdateAvailable(tt.current, tt.latest)
			if (err != nil) != tt.wantErr {
				t.Errorf("isUpdateAvailable() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.expected {
				t.Errorf("isUpdateAvailable() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestReadVersionFromPlistBytes(t *testing.T) {
	plist := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0">
<dict>
	<key>CFBundleName</key>
	<string>Waystile</string>
	<key>CFBundleShortVersionString</key>
	<string>0.1.0</string>
	<key>CFBundleVersion</key>
	<string>0.1.0</string>
</dict>
</plist>`)
	if got := readVersionFromPlistBytes(plist); got != "0.1.0" {
		t.Fatalf("readVersionFromPlistBytes() = %q, want 0.1.0", got)
	}
	if got := readVersionFromPlistBytes([]byte(`<key>CFBundleVersion</key><string>7</string><key>CFBundleShortVersionString</key><string>0.1.0</string><key>WaystileReleaseVersion</key><string>0.1.0-beta.2</string>`)); got != "0.1.0-beta.2" {
		t.Fatalf("release version must take precedence over marketing/build keys: %q", got)
	}
}

func TestIsCloudUpdateNewer(t *testing.T) {
	baseManifest := CloudReleaseManifest{
		SchemaVersion: 1,
		Version:       "0.1.0",
		CommitHash:    "51637ce",
		BuildDate:     "2026-10-28 13:23:32 UTC",
		PublishedAt:   "2026-10-28T13:30:00Z",
		ArchiveName:   "Waystile-Update.zip",
		SHA256:        strings.Repeat("a", 64),
		Size:          1024,
	}

	if !IsCloudUpdateNewer("0.0.9", "1e13dd4", "2026-10-25 16:57:23 UTC", baseManifest) {
		t.Fatal("expected newer semver to report update")
	}
	if !IsCloudUpdateNewer("0.1.0", "1e13dd4", "2026-10-25 16:57:23 UTC", baseManifest) {
		t.Fatal("expected same semver with newer build date and different commit to report update")
	}
	if IsCloudUpdateNewer("0.1.0", "51637ce", "2026-10-28 13:23:32 UTC", baseManifest) {
		t.Fatal("expected identical commit to not report update")
	}
	if IsCloudUpdateNewer("0.1.0", "9999999", "2026-10-29 13:23:32 UTC", baseManifest) {
		t.Fatal("expected older manifest build date to not overwrite newer local build")
	}
	if IsCloudUpdateNewer("0.2.0", "1e13dd4", "2026-10-25 16:57:23 UTC", baseManifest) {
		t.Fatal("expected older manifest semver to not report update")
	}
}

func TestValidateCloudManifestRejectsUnsafeInputs(t *testing.T) {
	valid := CloudReleaseManifest{
		SchemaVersion: 1,
		Version:       "0.1.0",
		CommitHash:    "51637ce",
		ArchiveName:   "Waystile-Update.zip",
		SHA256:        strings.Repeat("b", 64),
		Size:          4096,
		TeamID:        "ABCDE12345",
	}
	if err := ValidateCloudManifest(valid); err != nil {
		t.Fatalf("unexpected error on valid manifest: %v", err)
	}

	traversal := valid
	traversal.ArchiveName = "../Waystile-Update.zip"
	if err := ValidateCloudManifest(traversal); err == nil {
		t.Fatal("expected path traversal archive name to fail validation")
	}

	badHash := valid
	badHash.SHA256 = "not-a-sha256"
	if err := ValidateCloudManifest(badHash); err == nil {
		t.Fatal("expected malformed sha256 to fail validation")
	}
}

func TestPublishCheckAndInstallCloudUpdateEndToEnd(t *testing.T) {
	tmp := t.TempDir()
	cloudDir := filepath.Join(tmp, "cloud-setup")
	sourceDir := filepath.Join(tmp, "source")
	installDir := filepath.Join(tmp, "Applications")
	backupDir := filepath.Join(tmp, "backups")
	extSourceDir := filepath.Join(tmp, "ext-source")
	extTargetDir := filepath.Join(tmp, "ext-target")

	if err := os.MkdirAll(installDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(extSourceDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(extSourceDir, "manifest.json"), []byte(`{"name":"Waystile","version":"0.4.0"}`), 0644); err != nil {
		t.Fatal(err)
	}

	routerBundle := filepath.Join(sourceDir, "Waystile.app")
	if err := os.MkdirAll(filepath.Join(routerBundle, "Contents", "MacOS"), 0755); err != nil {
		t.Fatal(err)
	}
	plist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleExecutable</key>
	<string>Finicky</string>
	<key>CFBundleIdentifier</key>
	<string>com.kevinlee.finicky</string>
	<key>CFBundleName</key>
	<string>Waystile</string>
	<key>CFBundleShortVersionString</key>
	<string>0.2.0</string>
	<key>CFBundleVersion</key>
	<string>0.2.0</string>
</dict>
</plist>`
	if err := os.WriteFile(filepath.Join(routerBundle, "Contents", "Info.plist"), []byte(plist), 0644); err != nil {
		t.Fatal(err)
	}
	// Copy system /usr/bin/true as a valid Mach-O executable for codesign testing
	trueBytes, err := os.ReadFile("/usr/bin/true")
	if err != nil {
		t.Fatal(err)
	}
	execPath := filepath.Join(routerBundle, "Contents", "MacOS", "Finicky")
	if err := os.WriteFile(execPath, trueBytes, 0755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("codesign", "--force", "--deep", "--sign", "-", routerBundle).CombinedOutput(); err != nil {
		t.Fatalf("codesign failed: %s", string(out))
	}

	oldCommit, oldDate := commitHash, buildDate
	commitHash, buildDate = "51637ce", "2026-10-28 13:23:32 UTC"
	t.Cleanup(func() { commitHash, buildDate = oldCommit, oldDate })

	manifest, err := PublishCloudUpdate(PublishOptions{
		CloudSetupDir:      cloudDir,
		RouterAppPath:      routerBundle,
		ChromeExtensionDir: extSourceDir,
		AllowAdHocForTest:  true,
		AllowSingleArch:    true,
	})
	if err != nil {
		t.Fatalf("PublishCloudUpdate failed: %v", err)
	}
	if manifest.Version != "0.2.0" || !manifest.IncludesChromeExtension {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}

	info := CheckCloudUpdate(cloudDir, "0.1.0", "1e13dd4", "2026-10-25 16:57:23 UTC")
	if info == nil || !info.HasUpdate || !info.CanInstallAutomatically || info.Source != "icloud" {
		t.Fatalf("expected cloud update to be detected, got %+v", info)
	}
	oldRouter := filepath.Join(installDir, "Waystile.app")
	if err := os.MkdirAll(oldRouter, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldRouter, "previous-build"), []byte("keep this app"), 0600); err != nil {
		t.Fatal(err)
	}
	blockedBackup := filepath.Join(tmp, "backup-blocker")
	if err := os.WriteFile(blockedBackup, []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := InstallCloudUpdate(InstallOptions{CloudSetupDir: cloudDir, InstallDir: installDir, BackupRootDir: blockedBackup, AllowAdHocForTest: true, AllowSingleArch: true, SkipLSRegister: true}); err == nil {
		t.Fatal("backup failure must stop installation")
	}
	if contents, err := os.ReadFile(filepath.Join(oldRouter, "previous-build")); err != nil || string(contents) != "keep this app" {
		t.Fatalf("backup failure changed installed app: %q %v", contents, err)
	}

	result, err := InstallCloudUpdate(InstallOptions{
		CloudSetupDir:      cloudDir,
		InstallDir:         installDir,
		BackupRootDir:      backupDir,
		ChromeExtensionDir: extTargetDir,
		AllowAdHocForTest:  true,
		AllowSingleArch:    true,
		SkipLSRegister:     true,
	})
	if err != nil {
		t.Fatalf("InstallCloudUpdate failed: %v", err)
	}
	if result.Version != "0.2.0" || result.UpdatedChromeExtension {
		t.Fatalf("unexpected install result: %+v", result)
	}
	if _, err := os.Stat(filepath.Join(installDir, "Waystile.app", "Contents", "Info.plist")); err != nil {
		t.Fatalf("installed Waystile.app missing: %v", err)
	}
	if contents, err := os.ReadFile(filepath.Join(result.BackupDir, "Waystile.app", "previous-build")); err != nil || string(contents) != "keep this app" {
		t.Fatalf("previous app backup missing after successful activation: %q %v", contents, err)
	}
	if _, err := os.Stat(filepath.Join(extTargetDir, "manifest.json")); !os.IsNotExist(err) {
		t.Fatalf("unsigned Chrome extension must remain a manual install: %v", err)
	}

	// Tamper with the archive and verify InstallCloudUpdate rejects SHA-256 mismatch
	archivePath := filepath.Join(cloudDir, manifest.ArchiveName)
	f, err := os.OpenFile(archivePath, os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteAt([]byte("X"), 0)
	_ = f.Close()

	if _, err := InstallCloudUpdate(InstallOptions{
		CloudSetupDir:     cloudDir,
		InstallDir:        installDir,
		BackupRootDir:     backupDir,
		AllowAdHocForTest: true,
		AllowSingleArch:   true,
		SkipLSRegister:    true,
	}); err == nil || !strings.Contains(err.Error(), "SHA-256") {
		t.Fatalf("expected SHA-256 tamper error, got %v", err)
	}
}

func isolateUpdateCheck(t *testing.T, current string) {
	t.Helper()
	t.Setenv("FINICKY_UPDATE_CACHE_DIR", t.TempDir())
	t.Setenv("FINICKY_CLOUD_UPDATE_DIR", t.TempDir())
	t.Setenv("FINICKY_MOCK_UPDATE", "")
	oldVersion, oldHost := releaseVersion, apiHost
	releaseVersion, apiHost = current, ""
	t.Cleanup(func() { releaseVersion, apiHost = oldVersion, oldHost })
}

func TestGitHubUpdateChannelAndRouterAsset(t *testing.T) {
	isolateUpdateCheck(t, "0.1.0-beta.1")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[
		 {"tag_name":"v0.1.0-beta.2","prerelease":true,"html_url":"https://example.com/beta2","assets":[
		  {"name":"Waystile-0.1.0-beta.2-source.zip","browser_download_url":"https://example.com/source"},
		  {"name":"Waystile-Chrome-0.4.1.zip","browser_download_url":"https://example.com/chrome"},
		  {"name":"Waystile-Actions-0.1.0-beta.2-universal.zip","browser_download_url":"https://example.com/actions"},
		  {"name":"Waystile-0.1.0-beta.2-universal.zip","browser_download_url":"https://example.com/router"}]},
		 {"tag_name":"v0.0.9","html_url":"https://example.com/stable"},
		 {"tag_name":"v99.0.0","draft":true}
		]`))
	}))
	defer server.Close()
	t.Setenv("FINICKY_GITHUB_RELEASES_URL", server.URL)
	info := checkForUpdates(true)
	if info == nil || !info.HasUpdate || info.LatestVersion != "0.1.0-beta.2" || info.DownloadUrl != "https://example.com/router" {
		t.Fatalf("beta users must receive the newer beta router archive: %+v", info)
	}
	releaseVersion = "0.0.8"
	info = checkForUpdates(true)
	if info == nil || info.LatestVersion != "0.0.9" || info.DownloadUrl != "https://example.com/stable" {
		t.Fatalf("stable channel must exclude prereleases and fall back to release page: %+v", info)
	}
}

func TestFailedUpdateChecksAreCachedButManualCheckRetries(t *testing.T) {
	isolateUpdateCheck(t, "0.1.0-beta.1")
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1); w.WriteHeader(http.StatusNotFound) }))
	defer server.Close()
	t.Setenv("FINICKY_GITHUB_RELEASES_URL", server.URL)
	checkForUpdates(false)
	checkForUpdates(false)
	if requests.Load() != 1 {
		t.Fatalf("failed automatic check retried before cooldown: %d requests", requests.Load())
	}
	checkForUpdates(true)
	if requests.Load() != 2 {
		t.Fatalf("manual check must bypass cooldown: %d requests", requests.Load())
	}
}

func TestOlderCloudManifestDoesNotHidePublicUpdate(t *testing.T) {
	isolateUpdateCheck(t, "0.1.0-beta.1")
	cloudDir := os.Getenv("FINICKY_CLOUD_UPDATE_DIR")
	manifest := CloudReleaseManifest{SchemaVersion: 1, Version: "0.0.9", ArchiveName: "Waystile-Update.zip", SHA256: strings.Repeat("a", 64), Size: 1}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cloudDir, cloudManifestName), data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cloudDir, manifest.ArchiveName), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"tag_name":"v0.1.0-beta.2","prerelease":true}]`))
	}))
	defer server.Close()
	t.Setenv("FINICKY_GITHUB_RELEASES_URL", server.URL)
	for i := 0; i < 2; i++ {
		info := checkForUpdates(false)
		if info == nil || !info.HasUpdate || info.Source != "github" {
			t.Fatalf("cloud status hid public update (attempt %d): %+v", i, info)
		}
	}
}

func TestCloudUpdateNeverInstallsAnOlderBuild(t *testing.T) {
	manifest := CloudReleaseManifest{Version: "0.1.0", CommitHash: "abcdef0", BuildDate: time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)}
	if IsCloudUpdateNewer("0.1.0-beta.2", "1234567", time.Now().UTC().Format(time.RFC3339), manifest) {
		t.Fatal("older marketing-version-only build must not replace newer beta")
	}
}

func TestActivateBundlesRollsBackBothApps(t *testing.T) {
	root := t.TempDir()
	writeBundle := func(path, contents string) {
		t.Helper()
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "marker"), []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}
	router, companion := filepath.Join(root, "Waystile.app"), filepath.Join(root, "Waystile Actions.app")
	writeBundle(router, "old router")
	writeBundle(companion, "old companion")
	stagedRouter := filepath.Join(root, "staging", "Waystile.app")
	writeBundle(stagedRouter, "new router")
	err := activateBundles([]bundleReplacement{{stagedRouter, router}, {filepath.Join(root, "missing.app"), companion}})
	if err == nil {
		t.Fatal("expected activation failure for missing companion")
	}
	for path, want := range map[string]string{router: "old router", companion: "old companion"} {
		got, err := os.ReadFile(filepath.Join(path, "marker"))
		if err != nil || string(got) != want {
			t.Fatalf("previous app was not restored: %s %q %v", path, got, err)
		}
	}
}
