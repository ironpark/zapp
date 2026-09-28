package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/ironpark/zapp"
	"github.com/ironpark/zapp/pkg/upload"
	"github.com/urfave/cli/v3"
)

const uploadCategory = "[upload]"

// endpointFlags describe one upload endpoint, for `zapp upload` and for the
// endpoint `zapp build` adds to the project's.
func endpointFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{Category: uploadCategory, Name: "upload-url", Usage: "Endpoint to send artifacts to; ${file.name} is replaced by each file's name"},
		&cli.StringFlag{Category: uploadCategory, Name: "upload-method", Usage: "PUT sends the file as the body, POST as a multipart form field (default: PUT)"},
		&cli.StringFlag{Category: uploadCategory, Name: "upload-field", Usage: "Form field of a POST upload (default: file)"},
		&cli.StringSliceFlag{Category: uploadCategory, Name: "upload-header", Usage: `Header sent with each upload, as "Name: value"; ZAPP_UPLOAD_HEADER holds one per line`},
	}
}

// distributionFlags choose the build's zip and upload steps.
func distributionFlags() []cli.Flag {
	return append([]cli.Flag{
		&cli.BoolFlag{Name: "zip", Usage: "Archive the notarized app as a ZIP (--zip=false skips it)"},
		&cli.BoolFlag{Name: "no-upload", Usage: "Skip uploading"},
		&cli.StringSliceFlag{Category: uploadCategory, Name: "upload-artifacts", Usage: "Artifacts to send to --upload-url: zip, dmg, pkg (default: all built)"},
	}, endpointFlags()...)
}

// parseHeaders reads "Name: value" lines.
func parseHeaders(lines []string) (map[string]string, error) {
	headers := map[string]string{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		name, value, ok := strings.Cut(line, ":")
		if !ok || strings.TrimSpace(name) == "" {
			return nil, fmt.Errorf(`upload header %q must be written "Name: value"`, strings.SplitN(line, " ", 2)[0])
		}
		headers[strings.TrimSpace(name)] = strings.TrimSpace(value)
	}
	return headers, nil
}

func splitLines(v string) []string { return strings.Split(v, "\n") }

func splitWords(v string) []string {
	return strings.FieldsFunc(v, func(r rune) bool { return r == ',' || r == ' ' || r == '\n' })
}

// endpoint reads the endpoint flags, or their environment variables. Its URL
// is empty when none was given; the other endpoint flags then have nothing
// to describe.
func endpoint(c *cli.Command) (e zapp.UploadConfig, err error) {
	e.URL, _ = flagValue(c, "upload-url")
	e.Method, _ = flagValue(c, "upload-method")
	e.Field, _ = flagValue(c, "upload-field")
	if lines, _ := flagList(c, "upload-header", splitLines); len(lines) > 0 {
		if e.Headers, err = parseHeaders(lines); err != nil {
			return e, err
		}
	}
	if e.URL == "" && (e.Method != "" || e.Field != "" || len(e.Headers) > 0) {
		return e, fmt.Errorf("upload method, field and headers need --upload-url")
	}
	return e, nil
}

// overlayUpload adds the endpoint given on the command line to the project's
// and applies --zip and --no-upload.
func overlayUpload(c *cli.Command, p *zapp.Project) error {
	if b, ok, err := flagBool(c, "zip"); err != nil {
		return err
	} else if ok {
		if !b {
			p.Zip = nil
		} else if p.Zip == nil {
			p.Zip = &zapp.ZipConfig{}
		}
	}
	e, err := endpoint(c)
	if err != nil {
		return err
	}
	artifacts, _ := flagList(c, "upload-artifacts", splitWords)
	if e.URL != "" {
		e.Artifacts = artifacts
		p.Upload = append(p.Upload, e)
	} else if len(artifacts) > 0 {
		return fmt.Errorf("--upload-artifacts needs --upload-url")
	}
	if b, _, err := flagBool(c, "no-upload"); err != nil {
		return err
	} else if b {
		p.Upload = nil
	}
	return nil
}

func uploadCommand() *cli.Command {
	return &cli.Command{
		Name:      "upload",
		Usage:     "Send files to an HTTP endpoint",
		ArgsUsage: "<file> ...",
		Flags:     endpointFlags(),
		Action: func(ctx context.Context, c *cli.Command) error {
			if c.NArg() == 0 {
				return fmt.Errorf("name the files to upload")
			}
			e, err := endpoint(c)
			if err != nil {
				return err
			}
			if e.URL == "" {
				return fmt.Errorf("upload requires --upload-url")
			}
			for _, path := range c.Args().Slice() {
				where, err := upload.File(ctx, nil, e.Target(), path)
				if err != nil {
					return err
				}
				_, _ = fmt.Fprintf(c.Root().Writer, "%s -> %s\n", path, where)
			}
			return nil
		},
	}
}
