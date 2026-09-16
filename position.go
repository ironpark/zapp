package zapp

import (
	"encoding/json"
	"fmt"

	"github.com/goccy/go-yaml"
)

// Position is an icon center in Finder content coordinates: [x, y].
type Position [2]int

func (p *Position) UnmarshalJSON(data []byte) error {
	var values []*int
	if err := json.Unmarshal(data, &values); err != nil {
		return err
	}
	return p.assign(values)
}

func (p *Position) UnmarshalYAML(data []byte) error {
	data, err := yaml.YAMLToJSON(data)
	if err != nil {
		return err
	}
	return p.UnmarshalJSON(data)
}

func (p *Position) assign(values []*int) error {
	if len(values) != 2 || values[0] == nil || values[1] == nil {
		return fmt.Errorf("pos must contain exactly two integers: [x, y]")
	}
	*p = Position{*values[0], *values[1]}
	return nil
}
