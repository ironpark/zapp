package main

import (
	"fmt"
	"io"

	"github.com/fatih/color"
	"github.com/urfave/cli/v3"
)

type appLogger struct {
	Writer io.Writer
	Header string
}

// newAppLogger writes to the root command's writer.
func newAppLogger(app *cli.Command) *appLogger {
	return newLogger(app.Writer)
}

// newLogger writes to w. Subtasks invoked directly, rather than through the CLI
// parser, use this.
func newLogger(w io.Writer) *appLogger {
	return &appLogger{
		Writer: w,
		Header: color.HiCyanString("[ZAPP] "),
	}
}

// PrintValue emits best-effort diagnostic output, like CLI progress messages.
func (l *appLogger) PrintValue(key string, value any) {
	if value != "" {
		_, _ = fmt.Fprintf(l.Writer, "       %-30s: %v\n", color.HiWhiteString(key), value)
	}
}

func (l *appLogger) Success(format string, args ...any) (int, error) {
	str := color.HiGreenString(format, args...)
	return l.Println(str)
}

func (l *appLogger) Print(args ...any) (int, error) {
	return fmt.Fprint(l.Writer, append([]any{l.Header}, args...)...)
}

func (l *appLogger) Printf(format string, args ...any) (int, error) {
	return fmt.Fprintf(l.Writer, l.Header+format, args...)
}

func (l *appLogger) Println(args ...any) (int, error) {
	return l.Print(append(args, "\n")...)
}

func (l *appLogger) Warn(args ...any) (int, error) {
	return fmt.Fprint(l.Writer, append([]any{color.YellowString("[WARN] ")}, args...)...)
}

func (l *appLogger) Warnf(format string, args ...any) (int, error) {
	return fmt.Fprintf(l.Writer, color.YellowString("[WARN] ")+format, args...)
}

func (l *appLogger) Error(args ...any) (int, error) {
	return fmt.Fprint(l.Writer, append([]any{color.RedString("[ERROR] ")}, args...)...)
}

func (l *appLogger) Errorf(format string, args ...any) (int, error) {
	return fmt.Fprintf(l.Writer, color.RedString("[ERROR] ")+format, args...)
}
