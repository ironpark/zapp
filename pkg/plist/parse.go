// Package plist reads and writes property lists in the XML and binary
// ("bplist00") formats defined by CFPropertyList.
package plist

import (
	"fmt"
	"strings"
)

// Parse decodes a property list, accepting either the binary ("bplist00") or
// the XML representation, and returns it as Go values:
//
//	dict    map[string]interface{}
//	array   []interface{}
//	string  string
//	integer int64 (or uint64 when it overflows int64)
//	real    float64
//	bool    bool
//	date    time.Time
//	data    []byte
func Parse(data []byte) (interface{}, error) {
	if isBinary(data) {
		return parseBinary(data)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, fmt.Errorf("property list is empty")
	}
	return parseXML(data)
}

// ParseDict decodes a property list whose root object is a dictionary, which is
// the case for Info.plist and every other file this tool reads.
func ParseDict(data []byte) (map[string]interface{}, error) {
	value, err := Parse(data)
	if err != nil {
		return nil, err
	}
	dict, ok := value.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("property list root is %T, not a dictionary", value)
	}
	return dict, nil
}

// MarshalXML renders a value as an XML property list. Property lists are
// rewritten as XML regardless of the format they were read in, matching what
// plutil and PlistBuddy produce by default.
func MarshalXML(value interface{}) ([]byte, error) {
	return marshalXML(value)
}
