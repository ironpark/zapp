package plist

import (
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"
)

// The XML property list grammar implemented here follows Apple's
// PropertyList.dtd: a <plist> root holding exactly one object, where an object
// is one of dict, array, string, data, date, integer, real, true or false.

const (
	xmlHeader  = `<?xml version="1.0" encoding="UTF-8"?>` + "\n"
	xmlDoctype = `<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">` + "\n"

	// appleEpoch is the reference date of CFAbsoluteTime: 2001-01-01 00:00:00 UTC.
	appleEpochOffset = 978307200
)

// parseXML decodes an XML property list into Go values.
func parseXML(data []byte) (interface{}, error) {
	dec := xml.NewDecoder(strings.NewReader(string(data)))
	// Property lists reference an external DTD that we neither fetch nor need.
	dec.Strict = false
	dec.Entity = xml.HTMLEntity

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return nil, fmt.Errorf("no <plist> element found")
		}
		if err != nil {
			return nil, err
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if start.Name.Local != "plist" {
			// Some producers omit the <plist> wrapper; accept a bare object.
			return parseXMLElement(dec, start)
		}
		val, err := parseXMLPlistBody(dec)
		if err != nil {
			return nil, err
		}
		return val, nil
	}
}

// parseXMLPlistBody reads the single object contained in a <plist> element.
func parseXMLPlistBody(dec *xml.Decoder) (interface{}, error) {
	var value interface{}
	seen := false
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if seen {
				return nil, fmt.Errorf("<plist> must contain exactly one object")
			}
			value, err = parseXMLElement(dec, t)
			if err != nil {
				return nil, err
			}
			seen = true
		case xml.EndElement:
			if !seen {
				return nil, fmt.Errorf("<plist> is empty")
			}
			return value, nil
		}
	}
}

func parseXMLElement(dec *xml.Decoder, start xml.StartElement) (interface{}, error) {
	switch start.Name.Local {
	case "dict":
		return parseXMLDict(dec, start)
	case "array":
		return parseXMLArray(dec, start)
	case "true":
		return true, dec.Skip()
	case "false":
		return false, dec.Skip()
	case "string":
		return xmlText(dec, start)
	case "key":
		return nil, fmt.Errorf("<key> outside of a <dict>")
	case "integer":
		text, err := xmlText(dec, start)
		if err != nil {
			return nil, err
		}
		text = strings.TrimSpace(text)
		if n, err := strconv.ParseInt(text, 0, 64); err == nil {
			return n, nil
		}
		// CFPropertyList accepts unsigned values that overflow int64.
		n, err := strconv.ParseUint(text, 0, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid <integer> value %q", text)
		}
		return n, nil
	case "real":
		text, err := xmlText(dec, start)
		if err != nil {
			return nil, err
		}
		f, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
		if err != nil {
			return nil, fmt.Errorf("invalid <real> value %q", text)
		}
		return f, nil
	case "data":
		text, err := xmlText(dec, start)
		if err != nil {
			return nil, err
		}
		// Base64 payloads are conventionally wrapped across lines.
		raw, err := base64.StdEncoding.DecodeString(stripSpace(text))
		if err != nil {
			return nil, fmt.Errorf("invalid <data> value: %v", err)
		}
		return raw, nil
	case "date":
		text, err := xmlText(dec, start)
		if err != nil {
			return nil, err
		}
		ts, err := time.Parse(time.RFC3339, strings.TrimSpace(text))
		if err != nil {
			return nil, fmt.Errorf("invalid <date> value %q", text)
		}
		return ts, nil
	default:
		return nil, fmt.Errorf("unsupported plist element <%s>", start.Name.Local)
	}
}

func parseXMLDict(dec *xml.Decoder, start xml.StartElement) (map[string]interface{}, error) {
	dict := map[string]interface{}{}
	var key string
	haveKey := false
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "key" {
				if haveKey {
					return nil, fmt.Errorf("two consecutive <key> elements in <dict>")
				}
				key, err = xmlText(dec, t)
				if err != nil {
					return nil, err
				}
				haveKey = true
				continue
			}
			if !haveKey {
				return nil, fmt.Errorf("<%s> in <dict> is not preceded by a <key>", t.Name.Local)
			}
			value, err := parseXMLElement(dec, t)
			if err != nil {
				return nil, err
			}
			dict[key] = value
			haveKey = false
		case xml.EndElement:
			if haveKey {
				return nil, fmt.Errorf("<key>%s</key> has no value", key)
			}
			return dict, nil
		}
	}
}

func parseXMLArray(dec *xml.Decoder, start xml.StartElement) ([]interface{}, error) {
	array := []interface{}{}
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			value, err := parseXMLElement(dec, t)
			if err != nil {
				return nil, err
			}
			array = append(array, value)
		case xml.EndElement:
			return array, nil
		}
	}
}

// xmlText consumes an element and returns its character data.
func xmlText(dec *xml.Decoder, start xml.StartElement) (string, error) {
	var sb strings.Builder
	for {
		tok, err := dec.Token()
		if err != nil {
			return "", err
		}
		switch t := tok.(type) {
		case xml.CharData:
			sb.Write(t)
		case xml.StartElement:
			return "", fmt.Errorf("unexpected <%s> inside <%s>", t.Name.Local, start.Name.Local)
		case xml.EndElement:
			return sb.String(), nil
		}
	}
}

func stripSpace(s string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\t', '\n', '\r':
			return -1
		}
		return r
	}, s)
}

// marshalXML renders a value as an XML property list in the layout used by
// CFPropertyList: tab indentation and dictionary keys in sorted order so that
// rewriting a file produces a stable diff.
func marshalXML(value interface{}) ([]byte, error) {
	var sb strings.Builder
	sb.WriteString(xmlHeader)
	sb.WriteString(xmlDoctype)
	sb.WriteString(`<plist version="1.0">` + "\n")
	if err := writeXMLValue(&sb, value, 0); err != nil {
		return nil, err
	}
	sb.WriteString("</plist>\n")
	return []byte(sb.String()), nil
}

func writeXMLValue(sb *strings.Builder, value interface{}, depth int) error {
	indent := strings.Repeat("\t", depth)
	switch v := value.(type) {
	case nil:
		return fmt.Errorf("nil is not representable in a property list")
	case bool:
		if v {
			sb.WriteString(indent + "<true/>\n")
		} else {
			sb.WriteString(indent + "<false/>\n")
		}
	case string:
		sb.WriteString(indent + "<string>" + escapeXML(v) + "</string>\n")
	case []byte:
		sb.WriteString(indent + "<data>\n")
		encoded := base64.StdEncoding.EncodeToString(v)
		for i := 0; i < len(encoded); i += 68 {
			end := i + 68
			if end > len(encoded) {
				end = len(encoded)
			}
			sb.WriteString(indent + encoded[i:end] + "\n")
		}
		sb.WriteString(indent + "</data>\n")
	case time.Time:
		sb.WriteString(indent + "<date>" + v.UTC().Format("2006-01-02T15:04:05Z") + "</date>\n")
	case float32:
		return writeXMLValue(sb, float64(v), depth)
	case float64:
		sb.WriteString(indent + "<real>" + strconv.FormatFloat(v, 'g', -1, 64) + "</real>\n")
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		sb.WriteString(indent + "<integer>" + fmt.Sprintf("%d", v) + "</integer>\n")
	case []interface{}:
		if len(v) == 0 {
			sb.WriteString(indent + "<array/>\n")
			return nil
		}
		sb.WriteString(indent + "<array>\n")
		for _, item := range v {
			if err := writeXMLValue(sb, item, depth+1); err != nil {
				return err
			}
		}
		sb.WriteString(indent + "</array>\n")
	case map[string]interface{}:
		if len(v) == 0 {
			sb.WriteString(indent + "<dict/>\n")
			return nil
		}
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		sb.WriteString(indent + "<dict>\n")
		for _, k := range keys {
			sb.WriteString(indent + "\t<key>" + escapeXML(k) + "</key>\n")
			if err := writeXMLValue(sb, v[k], depth+1); err != nil {
				return err
			}
		}
		sb.WriteString(indent + "</dict>\n")
	default:
		return fmt.Errorf("unsupported plist value of type %T", value)
	}
	return nil
}

var xmlEscaper = strings.NewReplacer(
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
)

func escapeXML(s string) string { return xmlEscaper.Replace(s) }
