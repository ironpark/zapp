package plist

import (
	"fmt"

	"github.com/ironpark/zapp/pkg/appbundle"
	"github.com/ironpark/zapp/pkg/plist"
)

// open resolves what the user named -- a .plist file, or a bundle whose
// Info.plist is meant -- and reads it.
func open(path string) (*plist.Document, error) {
	plistPath, err := appbundle.FindPlistPath(path)
	if err != nil {
		return nil, err
	}
	return plist.Open(plistPath)
}

// edit reads the document at path, replaces the value at key with whatever
// change returns for the current one, and writes it back. Every command that
// modifies a property list goes through here, so they agree on how a key is
// found, how a container is written to, and how the file is replaced.
//
// change receives nil when the key does not exist yet.
func edit(path, key string, change func(existing any) (any, error)) error {
	doc, err := open(path)
	if err != nil {
		return err
	}
	container, name, existing, _ := plist.Resolve(doc.Root, key)
	value, err := change(existing)
	if err != nil {
		return fmt.Errorf("%s: %w", key, err)
	}
	if err := plist.Put(container, name, value); err != nil {
		return err
	}
	return doc.Save()
}

// mustExist is for commands that change a value rather than assign one, and so
// have nothing to work from when the key is absent.
func mustExist(key string, existing any) error {
	if existing == nil {
		return fmt.Errorf("key not found: %s", key)
	}
	return nil
}
