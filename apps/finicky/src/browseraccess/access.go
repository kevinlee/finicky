// Package browseraccess persists access to user-selected browser data folders.
// It only requests access through the system's native folder chooser.
package browseraccess

/*
#cgo CFLAGS: -x objective-c -fobjc-arc -fblocks
#cgo LDFLAGS: -framework Cocoa
#include <stdlib.h>
#include "access.h"
*/
import "C"

import (
	"errors"
	"sync"
	"sync/atomic"
	"unsafe"
)

var nextRequestID atomic.Uint64
var pending = struct {
	sync.Mutex
	requests map[uint64]chan error
}{requests: make(map[uint64]chan error)}

// Restore resolves previously granted folder bookmarks and keeps their read
// scopes active for browser profile discovery. It never presents permission UI.
// Invalid or revoked bookmarks return a useful error; other folders still restore.
func Restore() error {
	message := C.FinickyRestoreBrowserAccess()
	if message == nil {
		return nil
	}
	defer C.free(unsafe.Pointer(message))
	return errors.New(C.GoString(message))
}

// Request asynchronously asks the user to choose a browser's data folder.
// It produces exactly one error (nil for success), then closes the channel.
// The Cocoa main event loop must be running. Cancellation is an explicit error.
func Request() <-chan error {
	id := nextRequestID.Add(1)
	result := make(chan error, 1)
	pending.Lock()
	pending.requests[id] = result
	pending.Unlock()
	C.FinickyRequestBrowserAccess(C.ulonglong(id))
	return result
}

func complete(id uint64, err error) {
	pending.Lock()
	result, exists := pending.requests[id]
	delete(pending.requests, id)
	pending.Unlock()
	if exists {
		result <- err
		close(result)
	}
}

//export FinickyBrowserAccessFinished
func FinickyBrowserAccessFinished(id C.ulonglong, message *C.char) {
	var err error
	if message != nil {
		err = errors.New(C.GoString(message))
	}
	complete(uint64(id), err)
}
