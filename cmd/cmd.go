package cmd

import (
	"github.com/ironpark/zapp/cmd/dep"
	"github.com/ironpark/zapp/cmd/dmg"
	"github.com/ironpark/zapp/cmd/notarize"
	"github.com/ironpark/zapp/cmd/pkg"
	"github.com/ironpark/zapp/cmd/plist"
	"github.com/ironpark/zapp/cmd/sign"
	"github.com/urfave/cli/v2"
)

var commands = []*cli.Command{
	dmg.Command,
	pkg.Command,
	sign.Command,
	plist.Command,
	notarize.Command,
	dep.Command,
}

func GetCommands() []*cli.Command {
	return commands
}
