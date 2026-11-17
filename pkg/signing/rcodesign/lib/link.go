//go:build cgo && (linux || windows) && (amd64 || arm64)

package lib

/*
// Keep each archive before its system library dependencies.
#cgo linux,amd64 LDFLAGS: ${SRCDIR}/linux_amd64.a
#cgo linux,arm64 LDFLAGS: ${SRCDIR}/linux_arm64.a
#cgo linux LDFLAGS: -ldl -lpthread -lm
#cgo windows,amd64 LDFLAGS: ${SRCDIR}/windows_amd64.a
#cgo windows,arm64 LDFLAGS: ${SRCDIR}/windows_arm64.a
#cgo windows LDFLAGS: -static -lws2_32 -luserenv -lbcrypt -lntdll -lcrypt32 -lncrypt -lsecur32 -liphlpapi -lole32 -loleaut32 -lruntimeobject
*/
import "C"
