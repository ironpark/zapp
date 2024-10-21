// alias/create.go
package alias

import (
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"unicode/utf16"
)

func findVolume(startPath string, startStat os.FileInfo) (string, error) {
	lastDev := startStat.Sys().(*syscall.Stat_t).Dev
	lastIno := startStat.Sys().(*syscall.Stat_t).Ino
	lastPath := startPath

	for {
		parentPath := filepath.Dir(lastPath)
		parentStat, err := os.Stat(parentPath)
		if err != nil {
			return "", err
		}

		parentSys := parentStat.Sys().(*syscall.Stat_t)
		if parentSys.Dev != lastDev {
			return lastPath, nil
		}

		if parentSys.Ino == lastIno {
			return lastPath, nil
		}

		lastDev = parentSys.Dev
		lastIno = parentSys.Ino
		lastPath = parentPath
	}
}

func utf16be(str string) []byte {
	u16 := utf16.Encode([]rune(str))
	b := make([]byte, len(u16)*2)
	for i, v := range u16 {
		binary.BigEndian.PutUint16(b[i*2:], v)
	}
	return b
}

func Create(targetPath string) ([]byte, error) {
	info := Info{Version: 2, Extra: []Extra{}}

	parentPath := filepath.Dir(targetPath)
	targetStat, err := os.Stat(targetPath)
	if err != nil {
		return nil, err
	}
	parentStat, err := os.Stat(parentPath)
	if err != nil {
		return nil, err
	}
	volumePath, err := findVolume(targetPath, targetStat)
	if err != nil {
		return nil, err
	}
	volumeStat, err := os.Stat(volumePath)
	if err != nil {
		return nil, err
	}

	if !targetStat.IsDir() && !targetStat.Mode().IsRegular() {
		return nil, errors.New("target is not a file or directory")
	}

	targetSys := targetStat.Sys().(*syscall.Stat_t)
	parentSys := parentStat.Sys().(*syscall.Stat_t)

	info.Target.ID = uint32(targetSys.Ino)
	if targetStat.IsDir() {
		info.Target.Type = "directory"
	} else {
		info.Target.Type = "file"
	}
	info.Target.Filename = filepath.Base(targetPath)
	info.Target.Created = targetStat.ModTime()

	info.Parent.ID = uint32(parentSys.Ino)
	info.Parent.Name = filepath.Base(parentPath)

	volumeName, err := GetVolumeName(volumePath)
	if err != nil {
		return nil, err
	}
	info.Volume.Name = volumeName
	info.Volume.Created = volumeStat.ModTime()
	info.Volume.Signature = "H+"
	if volumePath == "/" {
		info.Volume.Type = "local"
	} else {
		info.Volume.Type = "other"
	}

	// Add Type 0
	info.Extra = append(info.Extra, Extra{
		Type:   0,
		Length: uint16(len(info.Parent.Name)),
		Data:   []byte(info.Parent.Name),
	})

	// Add Type 1
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, info.Parent.ID)
	info.Extra = append(info.Extra, Extra{
		Type:   1,
		Length: 4,
		Data:   b,
	})

	// Add Type 14
	filenameUTF16 := utf16be(info.Target.Filename)
	b = make([]byte, 2+len(filenameUTF16))
	binary.BigEndian.PutUint16(b, uint16(len(info.Target.Filename)))
	copy(b[2:], filenameUTF16)
	info.Extra = append(info.Extra, Extra{
		Type:   14,
		Length: uint16(len(b)),
		Data:   b,
	})

	// Add Type 15
	volumeNameUTF16 := utf16be(info.Volume.Name)
	b = make([]byte, 2+len(volumeNameUTF16))
	binary.BigEndian.PutUint16(b, uint16(len(info.Volume.Name)))
	copy(b[2:], volumeNameUTF16)
	info.Extra = append(info.Extra, Extra{
		Type:   15,
		Length: uint16(len(b)),
		Data:   b,
	})

	// Add Type 18
	if !filepath.HasPrefix(targetPath, volumePath) {
		return nil, errors.New("target path is not within volume path")
	}
	localPath := targetPath[len(volumePath):]
	info.Extra = append(info.Extra, Extra{
		Type:   18,
		Length: uint16(len(localPath)),
		Data:   []byte(localPath),
	})

	// Add Type 19
	info.Extra = append(info.Extra, Extra{
		Type:   19,
		Length: uint16(len(volumePath)),
		Data:   []byte(volumePath),
	})

	return Encode(info)
}
