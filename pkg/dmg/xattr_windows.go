package dmg

import "errors"

var errNoAttr = errors.New("Finder extended attributes are not available on Windows")

// Windows host files have no Finder metadata. The image tree still includes
// .VolumeIcon.icns and FinderInfo, written by the HFS+/APFS encoder.
func applyImageIcon(path string, icns []byte) error       { return nil }
func getXattr(path, name string, buf []byte) (int, error) { return 0, errNoAttr }
func setXattr(path, name string, data []byte) error       { return errNoAttr }
func sourceMetadata(path string, node *imageNode) error   { return nil }
