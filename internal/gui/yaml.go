package gui

import (
	"fmt"
	"io"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/ironpark/zapp"
)

func layoutYAML(c *zapp.DMGConfig) string {
	data, _ := yaml.Marshal(c)
	// An empty map means no items; omission means automatic app + Applications.
	if c.Contents != nil && len(c.Contents) == 0 {
		data = append(data, []byte("contents: {}\n")...)
	}
	return strings.TrimSuffix(string(data), "\n")
}

func (g *editor) yamlField() field {
	return field{Label: "DMG configuration", Value: layoutYAML(g.s.Project.DMG), Multiline: true, Syntax: "yaml", Height: g.yamlHeight(), Hint: "Ctrl/Cmd+Enter: apply · Esc: cancel", set: func(value string) error {
		var document map[string]any
		decoder := yaml.NewDecoder(strings.NewReader(value), yaml.Strict())
		if err := decoder.Decode(&document); err != nil {
			return err
		}
		if document == nil {
			return fmt.Errorf("enter a DMG settings mapping")
		}
		var extra any
		if err := decoder.Decode(&extra); err != io.EOF {
			return fmt.Errorf("enter exactly one YAML document")
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
