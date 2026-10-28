package version

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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
		TeamID:        "4F8Q446767",
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
	if result.Version != "0.2.0" || !result.UpdatedChromeExtension {
		t.Fatalf("unexpected install result: %+v", result)
	}
	if _, err := os.Stat(filepath.Join(installDir, "Waystile.app", "Contents", "Info.plist")); err != nil {
		t.Fatalf("installed Waystile.app missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(extTargetDir, "manifest.json")); err != nil {
		t.Fatalf("installed Chrome Extension missing: %v", err)
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
