package dmg

import (
	"fmt"
	"github.com/ironpark/zapp/cmd/zapp/internal/cli/project"
	"github.com/ironpark/zapp/pkg/dmg"
	"github.com/urfave/cli/v3"
	"strconv"
	"strings"
)

type position struct{ X, Y int }

func resolveConfig(c *cli.Command) (dmg.Config, string, error) {
	p, err := project.Load(c, "dmg")
	if err != nil {
		return dmg.Config{}, "", err
	}
	pl, err := p.Resolve()
	if err != nil {
		return dmg.Config{}, "", err
	}
	return *pl.DMG, pl.App, nil
}
func parsePosition(value string) (position, error) {
	parts := strings.Split(value, ",")
	if len(parts) != 2 {
		return position{}, fmt.Errorf("expected x,y")
	}
	x, e1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	y, e2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if e1 != nil || e2 != nil || x < 0 || y < 0 {
		return position{}, fmt.Errorf("expected nonnegative integer coordinates x,y")
	}
	return position{x, y}, nil
}
