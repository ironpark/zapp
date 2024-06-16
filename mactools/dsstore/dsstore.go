// Package dsstore is a package for writing .DS_Store files on macOS.
// Original code from https://github.com/LinusU/node-ds-store (MIT)
package dsstore

import (
	"bytes"
	"encoding/binary"
	"github.com/samber/lo"
	"os"
	"sort"
)

import (
	_ "embed"
)

//go:embed clean
var DSStoreClean []byte

type DSStore struct {
	Entries []*Entry
}

func NewDSStore() *DSStore {
	return &DSStore{
		Entries: make([]*Entry, 0),
	}
}

func (ds *DSStore) SetWindow(width, height, x, y int) {
	entry, ok := lo.Find(ds.Entries, func(entry *Entry) bool {
		return entry.EntryType == EntryTypeWorkspaceSettings
	})
	newEntry, err := NewWorkspaceSettingsEntry(".", x, y, width, height)
	if err != nil {
		panic(err)
	}
	if ok {
		entry.Buffer = newEntry.Buffer
	} else {
		ds.AddEntry(newEntry)
	}
}

func (ds *DSStore) SetIconPos(name string, x, y uint32) {
	entry, ok := lo.Find(ds.Entries, func(entry *Entry) bool {
		return entry.Filename == name && entry.EntryType == EntryTypeIconLocation
	})
	newEntry, err := NewIconLocationEntry(name, x, y)
	if err != nil {
		panic(err)
	}
	if ok {
		entry.Buffer = newEntry.Buffer
	} else {
		ds.AddEntry(newEntry)
	}
}

func (ds *DSStore) AddEntry(entry *Entry) {
	ds.Entries = append(ds.Entries, entry)
}

func (ds *DSStore) Write(filePath string) error {
	sort.Sort(Entries(ds.Entries))

	buf := bytes.Clone(DSStoreClean)
	modified := make([]byte, 3840)
	currentPos := 0
	P := uint32(0)
	count := uint32(len(ds.Entries))

	binary.BigEndian.PutUint32(modified[currentPos:], P)
	binary.BigEndian.PutUint32(modified[currentPos+4:], count)
	currentPos += 8

	for _, entry := range ds.Entries {
		copy(modified[currentPos:], entry.Buffer)
		currentPos += entry.Length()
	}

	binary.BigEndian.PutUint32(buf[76:], count)
	copy(buf[4100:], modified)
	return os.WriteFile(filePath, buf, 0644)
}
