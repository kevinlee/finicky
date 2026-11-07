package browser

/*
#cgo LDFLAGS: -lsqlite3
#include <sqlite3.h>
#include <stdlib.h>
*/
import "C"

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unsafe"
)

type firefoxProfile struct {
	name         string
	path         string
	absolutePath string
	modern       bool
}

var firefoxStoreID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// Mozilla's profile-group database schema and launch protocol:
// https://github.com/mozilla-firefox/firefox/blob/main/toolkit/profile/ProfilesDatastoreService.sys.mjs
// https://github.com/mozilla-firefox/firefox/blob/main/browser/components/profiles/SelectableProfileService.sys.mjs
// Only group IDs referenced by profiles.ini are opened, in read-only mode.
func readFirefoxProfileRecords(base string) []firefoxProfile {
	data, err := os.ReadFile(filepath.Join(base, "profiles.ini"))
	if err != nil {
		return nil
	}
	var records []firefoxProfile
	stores := map[string]bool{}
	var section, name, path, store string
	relative := true
	flush := func() {
		if !strings.HasPrefix(section, "Profile") {
			return
		}
		if name != "" && path != "" {
			abs := path
			if relative {
				abs = filepath.Join(base, path)
			}
			records = append(records, firefoxProfile{name: name, path: path, absolutePath: abs})
		}
		if firefoxStoreID.MatchString(store) {
			stores[store] = true
		}
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			flush()
			section = strings.Trim(line, "[]")
			name, path, store = "", "", ""
			relative = true
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch key {
		case "Name":
			name = value
		case "Path":
			path = value
		case "StoreID":
			store = value
		case "IsRelative":
			relative = value != "0"
		}
	}
	flush()
	byPath := map[string]firefoxProfile{}
	for _, record := range records {
		byPath[filepath.Clean(record.absolutePath)] = record
	}
	for store := range stores {
		rows, err := readFirefoxGroup(filepath.Join(base, "Profile Groups", store+".sqlite"))
		if err != nil {
			slog.Debug("Could not read Firefox profile group", "error", err)
			continue
		}
		for _, row := range rows {
			if row.name == "" || row.path == "" || filepath.IsAbs(row.path) || strings.ContainsRune(row.path, 0) {
				continue
			}
			row.absolutePath = filepath.Join(base, row.path)
			row.modern = true
			byPath[filepath.Clean(row.absolutePath)] = row
		}
	}
	result := make([]firefoxProfile, 0, len(byPath))
	for _, record := range byPath {
		result = append(result, record)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].name == result[j].name {
			return result[i].path < result[j].path
		}
		return result[i].name < result[j].name
	})
	return result
}

func readFirefoxGroup(path string) ([]firefoxProfile, error) {
	filename := C.CString(path)
	defer C.free(unsafe.Pointer(filename))
	var db *C.sqlite3
	if code := C.sqlite3_open_v2(filename, &db, C.SQLITE_OPEN_READONLY|C.SQLITE_OPEN_FULLMUTEX, nil); code != C.SQLITE_OK {
		if db != nil {
			C.sqlite3_close(db)
		}
		return nil, fmt.Errorf("could not open Firefox profile group (SQLite %d)", code)
	}
	defer C.sqlite3_close(db)
	C.sqlite3_busy_timeout(db, 200)
	query := C.CString(`SELECT name, path FROM Profiles ORDER BY id`)
	defer C.free(unsafe.Pointer(query))
	var stmt *C.sqlite3_stmt
	if code := C.sqlite3_prepare_v2(db, query, -1, &stmt, nil); code != C.SQLITE_OK {
		return nil, fmt.Errorf("unrecognized Firefox profile group schema (SQLite %d)", code)
	}
	defer C.sqlite3_finalize(stmt)
	result := []firefoxProfile{}
	for {
		code := C.sqlite3_step(stmt)
		if code == C.SQLITE_DONE {
			return result, nil
		}
		if code != C.SQLITE_ROW {
			return nil, fmt.Errorf("could not read Firefox profiles (SQLite %d)", code)
		}
		name := C.GoString((*C.char)(unsafe.Pointer(C.sqlite3_column_text(stmt, 0))))
		path := C.GoString((*C.char)(unsafe.Pointer(C.sqlite3_column_text(stmt, 1))))
		result = append(result, firefoxProfile{name: name, path: path, modern: true})
	}
}

func resolveFirefoxProfileArgs(base, profile string) ([]string, bool) {
	records := readFirefoxProfileRecords(base)
	var matches []firefoxProfile
	for _, record := range records {
		if record.path == profile || record.absolutePath == profile {
			matches = []firefoxProfile{record}
			break
		}
		if record.name == profile {
			matches = append(matches, record)
		}
	}
	if len(matches) != 1 {
		return nil, false
	}
	record := matches[0]
	if record.modern || record.path == profile || record.absolutePath == profile {
		info, err := os.Stat(record.absolutePath)
		if err != nil || !info.IsDir() {
			return nil, false
		}
		return []string{"--profile", record.absolutePath}, true
	}
	return []string{"-P", record.name}, true
}
