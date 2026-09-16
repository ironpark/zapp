//go:build cgo && (linux || windows) && (amd64 || arm64)

package rcodesign

/*
// Prebuilt archives ship in the Go module, so go install needs no download
// or generation step. Keep each archive before its system library dependencies.
#cgo linux,amd64 LDFLAGS: ${SRCDIR}/lib/linux_amd64.a
#cgo linux,arm64 LDFLAGS: ${SRCDIR}/lib/linux_arm64.a
#cgo linux LDFLAGS: -ldl -lpthread -lm
#cgo windows,amd64 LDFLAGS: ${SRCDIR}/lib/windows_amd64.a
#cgo windows,arm64 LDFLAGS: ${SRCDIR}/lib/windows_arm64.a
#cgo windows LDFLAGS: -static -lws2_32 -luserenv -lbcrypt -lntdll -lcrypt32 -lncrypt -lsecur32 -liphlpapi -lole32 -loleaut32 -lruntimeobject
#include <stdlib.h>
#include "zapp_rcodesign.h"
*/
import "C"

import (
	"context"
	"encoding/json"
	"errors"
	"unsafe"
)

func Available() error { return nil }

// invoke waits for Rust to finish, so no file mutation continues after return.
// Context cancellation is checked before entry; an in-flight FFI call cannot be interrupted.
func invoke(ctx context.Context, operation, path string, opts Options) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	data, err := json.Marshal(struct {
		Operation string  `json:"operation"`
		Path      string  `json:"path"`
		Options   Options `json:"options"`
	}{operation, path, opts})
	if err != nil {
		return err
	}
	request := C.CString(string(data))
	defer C.free(unsafe.Pointer(request))
	result := C.zapp_rcodesign_run(request)
	if result == nil {
		return nil
	}
	defer C.zapp_rcodesign_free(result)
	return errors.New(C.GoString(result))
}
