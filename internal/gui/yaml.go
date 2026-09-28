package gui

import (
	"fmt"
	"io"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/ironpark/zapp"
)

func layoutYAML(c *zapp.DMGConfig) string {
	text := marshalYAML(c)
	// An empty map means no items; omission means automatic app + Applications.
	if c.Contents != nil && len(c.Contents) == 0 {
		text += "\ncontents: {}"
	}
	return text
}

func marshalYAML(v any) string {
	data, _ := yaml.Marshal(v)
	return strings.TrimSuffix(string(data), "\n")
}

// decodeOneYAML decodes value, which must hold exactly one YAML document,
// into out.
func decodeOneYAML(value string, out any, opts ...yaml.DecodeOption) error {
	decoder := yaml.NewDecoder(strings.NewReader(value), opts...)
	if err := decoder.Decode(out); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return fmt.Errorf("enter exactly one YAML document")
	}
	return nil
}

func (g *editor) yamlField() field {
	return field{Label: "DMG configuration", Value: layoutYAML(g.s.Project.DMG), Multiline: true, Syntax: "yaml", Height: g.yamlHeight(), Hint: "Ctrl/Cmd+Enter: apply · Esc: cancel", set: func(value string) error {
		var document map[string]any
		if err := decodeOneYAML(value, &document, yaml.Strict()); err != nil {
			return err
		}
		if document == nil {
			return fmt.Errorf("enter a DMG settings mapping")
		}
		var next zapp.DMGConfig
		if err := yaml.UnmarshalWithOptions([]byte(value), &next, yaml.Strict()); err != nil {
			return err
		}
		g.s.Project.DMG = &next
		return nil
	}}
}

func (g *editor) yamlHeight() int { return max(108, g.contentBottom()-workspaceTop-125) }
