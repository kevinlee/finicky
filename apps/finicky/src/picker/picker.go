// Package picker presents the native destination chooser. It never opens URLs.
package picker

/*
#cgo CFLAGS: -x objective-c -fobjc-arc -fblocks
#cgo LDFLAGS: -framework Cocoa
#include <stdlib.h>
#include "picker.h"
*/
import "C"

import (
	"encoding/json"
	"finicky/browser"
	"strings"
	"sync"
	"sync/atomic"
	"unsafe"
)

type Destination struct {
	ID       string                `json:"id"`
	Name     string                `json:"name"`
	Subtitle string                `json:"subtitle"`
	Shortcut string                `json:"shortcut"`
	Color    string                `json:"color"`
	Browser  browser.BrowserConfig `json:"browser"`
}

type Request struct {
	URL          string        `json:"url"`
	Destinations []Destination `json:"destinations"`
	Private      bool          `json:"private"`
	NewWindow    bool          `json:"newWindow"`
	Background   bool          `json:"background"`
}

// Index is zero-based for destination choices and -1 for other actions.
// Copy and Share are terminal actions; the caller performs the requested action.
type Selection struct {
	Index      int
	Cancelled  bool
	Remember   bool
	Copy       bool
	Share      bool
	Private    bool
	NewWindow  bool
	Background bool
	// BackgroundSpecified distinguishes an unchecked control from a caller
	// that supplied no background preference.
	BackgroundSpecified bool
}

const (
	flagCancelled uint32 = 1 << iota
	flagRemember
	flagCopy
	flagShare
	flagPrivate
	flagNewWindow
	flagBackground
	flagBackgroundSpecified
)

var nextID atomic.Uint64
var results = struct {
	sync.Mutex
	pending map[uint64]chan Selection
}{pending: make(map[uint64]chan Selection)}

// Show queues a native picker without blocking. Every request produces exactly
// one result and then closes its channel. Multiple links are presented in order.
// The application's main Cocoa event loop must be running.
func Show(request Request) <-chan Selection {
	request = normalizedRequest(request)
	result := make(chan Selection, 1)
	data, err := json.Marshal(request)
	if err != nil {
		result <- Selection{Index: -1, Cancelled: true}
		close(result)
		return result
	}
	id := nextID.Add(1)
	results.Lock()
	results.pending[id] = result
	results.Unlock()
	encoded := C.CString(string(data))
	defer C.free(unsafe.Pointer(encoded))
	C.FinickyShowPicker(C.ulonglong(id), encoded)
	return result
}

// normalizedRequest copies user-supplied data, so preparing shortcuts cannot
// mutate saved destinations. Numbers remain reserved for positional shortcuts.
func normalizedRequest(request Request) Request {
	request.Destinations = append([]Destination(nil), request.Destinations...)
	used := make(map[string]bool)
	for i := range request.Destinations {
		destination := &request.Destinations[i]
		if strings.TrimSpace(destination.Name) == "" {
			destination.Name = destination.Browser.Name
		}
		shortcut := strings.ToLower(strings.TrimSpace(destination.Shortcut))
		if len(shortcut) != 1 || shortcut[0] < 'a' || shortcut[0] > 'z' || used[shortcut] {
			shortcut = ""
		}
		destination.Shortcut = shortcut
		if shortcut != "" {
			used[shortcut] = true
		}
	}
	return request
}

func complete(id uint64, selection Selection) {
	results.Lock()
	channel, found := results.pending[id]
	delete(results.pending, id)
	results.Unlock()
	if found {
		channel <- selection
		close(channel)
	}
}

//export FinickyPickerDidSelect
func FinickyPickerDidSelect(id C.ulonglong, index C.int, flags C.uint) {
	f := uint32(flags)
	complete(uint64(id), Selection{
		Index: int(index), Cancelled: f&flagCancelled != 0,
		Remember: f&flagRemember != 0, Copy: f&flagCopy != 0,
		Share: f&flagShare != 0, Private: f&flagPrivate != 0,
		NewWindow: f&flagNewWindow != 0, Background: f&flagBackground != 0,
		BackgroundSpecified: f&flagBackgroundSpecified != 0,
	})
}
