//go:build darwin

package settingssync

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Foundation
#include <stdlib.h>
char *FSSRead(const char *path, int *code);
char *FSSList(const char *path, int *code);
char *FSSWrite(const char *path, const void *data, int length, int *code);
char *FSSMkdir(const char *path, int *code);
*/
import "C"

import (
	"encoding/json"
	"fmt"
	"unsafe"
)

type cloudFailure struct{ state, message string }

func (e *cloudFailure) Error() string { return e.message }

func cloudResult(value *C.char, code C.int) ([]byte, error) {
	if value == nil {
		return nil, &cloudFailure{"offline", "Cloud storage is unavailable"}
	}
	defer C.free(unsafe.Pointer(value))
	result := []byte(C.GoString(value))
	if code != 0 {
		state := "offline"
		if code == 1 {
			state = "pending"
		}
		if code == 3 {
			state = "missing"
		}
		if code == 4 {
			state = "conflict"
		}
		return nil, &cloudFailure{state, string(result)}
	}
	return result, nil
}
func cloudRead(path string) ([]byte, error) {
	p := C.CString(path)
	defer C.free(unsafe.Pointer(p))
	var code C.int
	value := C.FSSRead(p, &code)
	return cloudResult(value, code)
}
func cloudList(path string) ([]string, error) {
	p := C.CString(path)
	defer C.free(unsafe.Pointer(p))
	var code C.int
	value := C.FSSList(p, &code)
	result, err := cloudResult(value, code)
	if err != nil {
		return nil, err
	}
	var names []string
	if err := json.Unmarshal(result, &names); err != nil {
		return nil, fmt.Errorf("invalid coordinated directory listing")
	}
	return names, nil
}
func cloudWrite(path string, data []byte) error {
	p := C.CString(path)
	defer C.free(unsafe.Pointer(p))
	var code C.int
	var pointer unsafe.Pointer
	if len(data) > 0 {
		pointer = unsafe.Pointer(&data[0])
	}
	value := C.FSSWrite(p, pointer, C.int(len(data)), &code)
	_, err := cloudResult(value, code)
	return err
}
func cloudMkdir(path string) error {
	p := C.CString(path)
	defer C.free(unsafe.Pointer(p))
	var code C.int
	value := C.FSSMkdir(p, &code)
	_, err := cloudResult(value, code)
	return err
}
