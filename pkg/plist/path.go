package plist

import (
	"fmt"
	"strconv"
	"strings"
)

// A key path addresses a value inside nested containers, so the arbitrary
// loads flag of an Info.plist is NSAppTransportSecurity.NSAllowsArbitraryLoads.
//
// Many real keys contain dots of their own -- com.apple.security.app-sandbox is
// one key, not a path of four -- so at each step the longest prefix that names
// something wins, and a path is only split where no such key exists. An element
// of an array is addressed by its index.

// Resolve walks path from root. It reports the container holding the final
// component and that component's name, whether or not it exists, so a caller
// can read the value, replace it, or create it. found says whether it was
// there; value is its current contents when it was.
func Resolve(root map[string]any, path string) (container any, name string, value any, found bool) {
	container = root
	for {
		name, value, remainder, ok := step(container, path)
		if !ok {
			// Nothing here matches. The caller may still create the whole
			// remaining path as one literal key in this container.
			return container, path, nil, false
		}
		if remainder == "" {
			return container, name, value, true
		}
		container, path = value, remainder
	}
}

// step returns the element of container named by the longest prefix of path
// that exists, and whatever is left of the path after it.
func step(container any, path string) (name string, value any, remainder string, ok bool) {
	for cut := len(path); cut > 0; cut = strings.LastIndex(path[:cut], ".") {
		name = path[:cut]
		if value, ok = lookup(container, name); ok {
			return name, value, strings.TrimPrefix(path[cut:], "."), true
		}
	}
	return "", nil, "", false
}

// lookup reads one named element of a dictionary or array.
func lookup(container any, name string) (any, bool) {
	switch c := container.(type) {
	case map[string]any:
		value, ok := c[name]
		return value, ok
	case []any:
		i, err := strconv.Atoi(name)
		if err != nil || i < 0 || i >= len(c) {
			return nil, false
		}
		return c[i], true
	}
	return nil, false
}

// Put stores value as the named element of container, which Resolve returned.
func Put(container any, name string, value any) error {
	switch c := container.(type) {
	case map[string]any:
		c[name] = value
		return nil
	case []any:
		i, err := strconv.Atoi(name)
		if err != nil || i < 0 || i >= len(c) {
			return fmt.Errorf("%q is not an index into an array of %d", name, len(c))
		}
		c[i] = value
		return nil
	}
	return fmt.Errorf("cannot set %s inside a %s", name, Kind(container))
}

// Delete removes the named element of container. An array is left alone, since
// removing an element would renumber every key path after it.
func Delete(container any, name string) error {
	dict, ok := container.(map[string]any)
	if !ok {
		return fmt.Errorf("cannot remove %s from a %s", name, Kind(container))
	}
	delete(dict, name)
	return nil
}

// Kind names a value the way a property list does, for error messages.
func Kind(value any) string {
	switch value.(type) {
	case map[string]any:
		return "dictionary"
	case []any:
		return "array"
	case nil:
		return "nothing"
	}
	return fmt.Sprintf("%T", value)
}
