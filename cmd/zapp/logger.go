package main

import (
	"fmt"
	"io"

	"github.com/fatih/color"
	"github.com/urfave/cli/v3"
)

// appLogger tags the progress a build reports so it stands apart from whatever
// the tools zapp drives print themselves. zapp.Logger asks for Printf alone.
type appLogger struct {
	Writer io.Writer
	Header string
}

// newAppLogger writes to the root command's writer.
func newAppLogger(app *cli.Command) *appLogger {
	return &appLogger{Writer: app.Writer, Header: color.HiCyanString("[ZAPP] ")}
}

func (l *appLogger) Printf(format string, args ...any) (int, error) {
	return fmt.Fprintf(l.Writer, l.Header+format, args...)
}
