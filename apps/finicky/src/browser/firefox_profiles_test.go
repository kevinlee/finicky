package browser

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

func firefoxFixture(t *testing.T, ini, sql string) string {
	t.Helper()
	base := t.TempDir()
	if err := os.WriteFile(filepath.Join(base, "profiles.ini"), []byte(ini), 0600); err != nil {
		t.Fatal(err)
	}
	for _, directory := range []string{"Profile Groups", "Profiles/anchor", "Profiles/work", "Profiles/other"} {
		if err := os.MkdirAll(filepath.Join(base, directory), 0700); err != nil {
			t.Fatal(err)
		}
	}
	if sql != "" {
		output, err := exec.Command("/usr/bin/sqlite3", filepath.Join(base, "Profile Groups/abcd1234.sqlite"), sql).CombinedOutput()
		if err != nil {
			t.Fatalf("create SQLite fixture: %v: %s", err, output)
		}
	}
	return base
}

func TestFirefoxProfileGroupsUseStablePaths(t *testing.T) {
	base := firefoxFixture(t, "[Profile0]\nName=old-name\nIsRelative=1\nPath=Profiles/anchor\nStoreID=abcd1234\n", `CREATE TABLE Profiles (id INTEGER PRIMARY KEY, name TEXT, path TEXT); INSERT INTO Profiles VALUES(1,'Personal','Profiles/anchor'),(2,'Work','Profiles/work');`)
	profiles := readBrowserProfiles(browserInfo{ID: "org.mozilla.firefox", Type: "Firefox"}, base)
	want := []BrowserProfile{
		{ID: "org.mozilla.firefox:Profiles/anchor", Name: "Personal", Profile: "Profiles/anchor"},
		{ID: "org.mozilla.firefox:Profiles/work", Name: "Work", Profile: "Profiles/work"},
	}
	if !reflect.DeepEqual(profiles, want) {
		t.Fatalf("profiles = %#v", profiles)
	}
	for _, selector := range []string{"Work", "Profiles/work", filepath.Join(base, "Profiles/work")} {
		args, ok := resolveFirefoxProfileArgs(base, selector)
		if !ok || !reflect.DeepEqual(args, []string{"--profile", filepath.Join(base, "Profiles/work")}) {
			t.Fatalf("%q: %v %v", selector, args, ok)
		}
	}
}

func TestFirefoxGroupDuplicateNamesRequireStablePath(t *testing.T) {
	base := firefoxFixture(t, "[Profile0]\nName=anchor\nPath=Profiles/anchor\nStoreID=abcd1234\n", `CREATE TABLE Profiles (id INTEGER PRIMARY KEY, name TEXT, path TEXT); INSERT INTO Profiles VALUES(1,'Work','Profiles/work'),(2,'Work','Profiles/other');`)
	if args, ok := resolveFirefoxProfileArgs(base, "Work"); ok {
		t.Fatalf("ambiguous name accepted: %v", args)
	}
	if args, ok := resolveFirefoxProfileArgs(base, "Profiles/other"); !ok || args[1] != filepath.Join(base, "Profiles/other") {
		t.Fatalf("stable path: %v %v", args, ok)
	}
	if err := os.Remove(filepath.Join(base, "Profiles/work")); err != nil {
		t.Fatal(err)
	}
	if _, ok := resolveFirefoxProfileArgs(base, "Profiles/work"); ok {
		t.Fatal("missing directory accepted")
	}
}

func TestFirefoxGroupFailureRetainsLegacyAndNeverCreatesDatabase(t *testing.T) {
	base := firefoxFixture(t, "[Profile0]\nName=Legacy\nPath=Profiles/anchor\nStoreID=missing\n", "")
	args, ok := resolveFirefoxProfileArgs(base, "Legacy")
	if !ok || !reflect.DeepEqual(args, []string{"-P", "Legacy"}) {
		t.Fatalf("legacy fallback: %v %v", args, ok)
	}
	if _, err := os.Stat(filepath.Join(base, "Profile Groups/missing.sqlite")); !os.IsNotExist(err) {
		t.Fatalf("read-only discovery created database: %v", err)
	}
	if _, ok := resolveFirefoxProfileArgs(base, "Unknown"); ok {
		t.Fatal("unknown profile accepted")
	}
}

func TestFirefoxGroupIgnoresUnreferencedStoresAndInvalidIDs(t *testing.T) {
	base := firefoxFixture(t, "[Profile0]\nName=Legacy\nPath=Profiles/anchor\nStoreID=../abcd1234\n", `CREATE TABLE Profiles (id INTEGER PRIMARY KEY, name TEXT, path TEXT); INSERT INTO Profiles VALUES(1,'Work','Profiles/work');`)
	profiles := readFirefoxProfileRecords(base)
	if len(profiles) != 1 || profiles[0].name != "Legacy" {
		t.Fatalf("unexpected store discovery: %#v", profiles)
	}
}
