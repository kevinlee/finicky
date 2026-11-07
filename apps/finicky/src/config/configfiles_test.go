package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestMissingDefaultConfigAllowsVisualSettingsButKeepsWatching(t *testing.T) {
	paths := configPathsForHome(t.TempDir(), "")
	if len(paths) != 6 {
		t.Fatalf("default search paths = %v", paths)
	}
	watcher := &ConfigFileWatcher{}
	// GetConfigPath must still signal absence so StartWatching watches parents.
	if path, err := watcher.getConfigPath(paths, false); path != "" || !errors.Is(err, errNoDefaultConfig) {
		t.Fatalf("missing default lookup = %q, %v", path, err)
	}
	if bundle, path, err := watcher.bundleConfig(paths); bundle != "" || path != "" || err != nil {
		t.Fatalf("visual-only startup = %q, %q, %v", bundle, path, err)
	}
}

func TestDefaultSearchFindsLaterTypeScriptConfig(t *testing.T) {
	paths := configPathsForHome(t.TempDir(), "")
	wanted := paths[len(paths)-1]
	if err := os.MkdirAll(filepath.Dir(wanted), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(wanted, []byte("export default {};"), 0600); err != nil {
		t.Fatal(err)
	}
	if got, err := (&ConfigFileWatcher{}).getConfigPath(paths, false); got != wanted || err != nil {
		t.Fatalf("later default config = %q, %v", got, err)
	}
}

func TestExplicitMissingConfigRemainsAnError(t *testing.T) {
	custom := filepath.Join(t.TempDir(), "requested.js")
	watcher := &ConfigFileWatcher{customConfigPath: custom}
	paths := configPathsForHome(t.TempDir(), custom)
	if _, err := watcher.getConfigPath(paths, false); err == nil || errors.Is(err, errNoDefaultConfig) {
		t.Fatalf("missing explicit config lookup = %v", err)
	}
	if bundle, path, err := watcher.bundleConfig(paths); bundle != "" || path != "" || err == nil {
		t.Fatalf("missing explicit config startup = %q, %q, %v", bundle, path, err)
	}
}

func TestExistingBrokenDefaultConfigDoesNotFallBack(t *testing.T) {
	for _, kind := range []string{"directory", "dangling symlink"} {
		t.Run(kind, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), ".finicky.js")
			var err error
			if kind == "directory" {
				err = os.Mkdir(path, 0700)
			} else {
				err = os.Symlink(filepath.Join(t.TempDir(), "missing.js"), path)
			}
			if err != nil {
				t.Fatal(err)
			}
			watcher := &ConfigFileWatcher{}
			if _, _, err := watcher.bundleConfig([]string{path}); err == nil || errors.Is(err, errNoDefaultConfig) {
				t.Fatalf("broken default config startup = %v", err)
			}
		})
	}
}

func TestConfigInspectionPermissionErrorDoesNotFallBack(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permissions")
	}
	directory := filepath.Join(t.TempDir(), "inaccessible")
	if err := os.Mkdir(directory, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(directory, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(directory, 0700) })
	watcher := &ConfigFileWatcher{}
	if _, _, err := watcher.bundleConfig([]string{filepath.Join(directory, "finicky.js")}); !errors.Is(err, os.ErrPermission) {
		t.Fatalf("inaccessible default config startup = %v", err)
	}
}

func TestUnreadableConfigCannotUseCachedBundle(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses file permissions")
	}
	directory := t.TempDir()
	path, cachedBundle := filepath.Join(directory, "finicky.js"), filepath.Join(directory, "bundle.js")
	if err := os.WriteFile(path, []byte("export default {};"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cachedBundle, []byte("cached"), 0600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	watcher := &ConfigFileWatcher{cache: &ConfigCache{cachedConfigPath: path, cachedBundlePath: cachedBundle, cachedModTime: info.ModTime()}}
	if bundle, _, err := watcher.bundleConfig([]string{path}); bundle != cachedBundle || err != nil {
		t.Fatalf("readable cached config = %q, %v", bundle, err)
	}
	if err := os.Chmod(path, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0600) })
	if bundle, _, err := watcher.bundleConfig([]string{path}); bundle != "" || !errors.Is(err, os.ErrPermission) {
		t.Fatalf("unreadable cached config = %q, %v", bundle, err)
	}
}

func TestInvalidDefaultJavaScriptRemainsAnError(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".finicky.js")
	if err := os.WriteFile(path, []byte("export default { broken: ;"), 0600); err != nil {
		t.Fatal(err)
	}
	// Empty in-memory cache avoids reading or writing the user's real cache.
	watcher := &ConfigFileWatcher{cache: &ConfigCache{}}
	if bundle, configPath, err := watcher.bundleConfig([]string{path}); bundle != "" || configPath != path || err == nil || errors.Is(err, errNoDefaultConfig) {
		t.Fatalf("invalid JavaScript startup = %q, %q, %v", bundle, configPath, err)
	}
}
