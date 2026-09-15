package macpkg

// payloadStat is the file metadata a CPIO payload entry needs, in the one shape
// the shared walk consumes. Each platform converts its own stat into it, so the
// contract between them is checked by the compiler rather than by field names
// happening to line up.
type payloadStat struct {
	Mode, Uid, Gid uint32
	Dev, Ino       uint64
}
