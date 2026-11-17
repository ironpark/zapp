//go:build cgo && (linux || windows) && (amd64 || arm64)

package rcodesign

/*
// The prebuilt archive the declarations below resolve against, and the system
// libraries it needs, are linked by the lib module imported below. It is a
// module of its own so that only this build constraint downloads it; go
// install needs no download or generation step beyond the module itself.
#include <stdlib.h>
#include "zapp_rcodesign.h"
*/
import "C"

import (
	"context"
	"encoding/json"
	"errors"
	"unsafe"

	_ "github.com/ironpark/zapp/pkg/signing/rcodesign/lib"
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
