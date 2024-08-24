package cmd

import "github.com/urfave/cli/v2"

var commands []*cli.Command

func GetCommands() []*cli.Command {
	return commands
}
