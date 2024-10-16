package cmd

import (
	"fmt"
	"github.com/fatih/color"
	"github.com/urfave/cli/v2"
	"io"
)

type AppLogger struct {
	Writer io.Writer
	Header string
}

func NewAppLogger(app *cli.App) *AppLogger {
	return &AppLogger{
		Writer: app.Writer,
		Header: color.HiCyanString("[ZAPP] "),
	}
}

func (l *AppLogger) PrintValue(key string, value any) {
	if value != "" {
		fmt.Fprintf(l.Writer, "       %-30s: %v\n", color.HiWhiteString(key), value)
	}
}

func (l *AppLogger) Success(format string, args ...any) (int, error) {
	str := color.HiGreenString(format, args...)
	return l.Println(str)
}

func (l *AppLogger) Print(args ...any) (int, error) {
	return fmt.Fprint(l.Writer, append([]any{l.Header}, args...)...)
}

func (l *AppLogger) Printf(format string, args ...any) (int, error) {
	return fmt.Fprintf(l.Writer, l.Header+format, args...)
}

func (l *AppLogger) Println(args ...any) (int, error) {
	return l.Print(append(args, "\n")...)
}

func (l *AppLogger) Warn(args ...any) (int, error) {
	return fmt.Fprint(l.Writer, append([]any{color.YellowString("[WARN] ")}, args...)...)
}

func (l *AppLogger) Warnf(format string, args ...any) (int, error) {
	return fmt.Fprintf(l.Writer, color.YellowString("[WARN] ")+format, args...)
}

func (l *AppLogger) Error(args ...any) (int, error) {
	return fmt.Fprint(l.Writer, append([]any{color.RedString("[ERROR] ")}, args...)...)
}

func (l *AppLogger) Errorf(format string, args ...any) (int, error) {
	return fmt.Fprintf(l.Writer, color.RedString("[ERROR] ")+format, args...)
}
