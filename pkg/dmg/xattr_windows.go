package dmg

import (
	"errors"
	"io/fs"

	"github.com/ironpark/zapp/pkg/macfs"
)

// Windows host files carry no Finder metadata, so every attribute reads as
// absent and the shared import path turns into a no-op. The image tree still
// includes .VolumeIcon.icns and FinderInfo, written by the HFS+/APFS encoder.
var errNoAttr = errors.New("Finder extended attributes are not available on Windows")

func absentAttr(err error) bool { return errors.Is(err, errNoAttr) }

func applyImageIcon(path string, icns []byte) error { return nil }

func getXattr(path, name string, buf []byte) (int, error) { return 0, errNoAttr }

func getSourceXattr(path, name string, buf []byte) (int, error) { return 0, errNoAttr }

func sourceResourceFork(path string, size int) (macfs.Source, error) {
	return nil, fs.ErrNotExist
}

func setXattr(path, name string, data []byte) error { return errNoAttr }
