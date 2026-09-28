package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/ironpark/zapp"
	"github.com/urfave/cli/v3"
)

const uploadCategory = "[upload]"

// endpointFlags describe an upload endpoint and a GitHub release, for `zapp
// upload` and for the ones `zapp build` adds to the project's.
func endpointFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{Category: uploadCategory, Name: "github-release", Usage: "Tag of a GitHub release to add artifacts to, created if missing; needs GITHUB_TOKEN"},
		&cli.StringFlag{Category: uploadCategory, Name: "github-repo", Usage: "owner/name of the --github-release repository (default: $GITHUB_REPOSITORY)"},
		&cli.StringFlag{Category: uploadCategory, Name: "upload-url", Usage: "Endpoint to send artifacts to; ${file.name} is replaced by each file's name"},
		&cli.StringFlag{Category: uploadCategory, Name: "upload-method", Usage: "PUT sends the file as the body, POST as a multipart form field (default: PUT)"},
		&cli.StringFlag{Category: uploadCategory, Name: "upload-field", Usage: "Form field of a POST upload (default: file)"},
		&cli.StringSliceFlag{Category: uploadCategory, Name: "upload-header", Usage: `Header sent with each upload, as "Name: value"; ZAPP_UPLOAD_HEADER holds one per line`},
	}
}

// distributionFlags choose the build's zip, checksums and upload steps.
func distributionFlags() []cli.Flag {
	return append([]cli.Flag{
		&cli.BoolFlag{Name: "zip", Usage: "Archive the notarized app as a ZIP (--zip=false skips it)"},
		&cli.BoolFlag{Name: "checksums", Usage: "List the SHA-256 of the ZIP, DMG and PKG built (--checksums=false skips it)"},
		&cli.BoolFlag{Name: "no-upload", Usage: "Skip uploading"},
		&cli.StringSliceFlag{Category: uploadCategory, Name: "upload-artifacts", Usage: "Artifacts to send to --upload-url and --github-release: zip, dmg, pkg, checksums (default: all built)"},
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

// endpoints reads the endpoint flags, or their environment variables: an
// HTTP endpoint, a GitHub release, both or neither.
func endpoints(c *cli.Command) ([]zapp.UploadConfig, error) {
	var list []zapp.UploadConfig
	var e zapp.UploadConfig
	e.URL, _ = flagValue(c, "upload-url")
	e.Method, _ = flagValue(c, "upload-method")
	e.Field, _ = flagValue(c, "upload-field")
	if lines, _ := flagList(c, "upload-header", splitLines); len(lines) > 0 {
		var err error
		if e.Headers, err = parseHeaders(lines); err != nil {
			return nil, err
		}
	}
	if e.URL != "" {
		list = append(list, e)
	} else if e.Method != "" || e.Field != "" || len(e.Headers) > 0 {
		return nil, fmt.Errorf("upload method, field and headers need --upload-url")
	}
	tag, _ := flagValue(c, "github-release")
	repo, _ := flagValue(c, "github-repo")
	if tag != "" {
		list = append(list, zapp.UploadConfig{GitHub: &zapp.GitHubRelease{Repo: repo, Tag: tag}})
	} else if repo != "" {
		return nil, fmt.Errorf("--github-repo needs --github-release")
	}
	return list, nil
}

// overlayUpload adds the endpoints given on the command line to the
// project's and applies --zip, --checksums and --no-upload.
func overlayUpload(c *cli.Command, p *zapp.Project) error {
	if err := toggle(c, "zip", &p.Zip); err != nil {
		return err
	}
	if err := toggle(c, "checksums", &p.Checksums); err != nil {
		return err
	}
	list, err := endpoints(c)
	if err != nil {
		return err
	}
	artifacts, _ := flagList(c, "upload-artifacts", splitWords)
	if len(list) == 0 && len(artifacts) > 0 {
		return fmt.Errorf("--upload-artifacts needs --upload-url or --github-release")
	}
	for _, e := range list {
		e.Artifacts = artifacts
		p.Upload = append(p.Upload, e)
	}
	if b, _, err := flagBool(c, "no-upload"); err != nil {
		return err
	} else if b {
		p.Upload = nil
	}
	return nil
}

// toggle applies a boolean flag that turns a section on, with its defaults,
// or off.
func toggle[T any](c *cli.Command, flag string, section **T) error {
	b, ok, err := flagBool(c, flag)
	switch {
	case err != nil:
		return err
	case !ok:
	case !b:
		*section = nil
	case *section == nil:
		*section = new(T)
	}
	return nil
}

func uploadCommand() *cli.Command {
	return &cli.Command{
		Name:      "upload",
		Usage:     "Send files to an HTTP endpoint or a GitHub release",
		ArgsUsage: "<file> ...",
		Flags:     endpointFlags(),
		Action: func(ctx context.Context, c *cli.Command) error {
			if c.NArg() == 0 {
				return fmt.Errorf("name the files to upload")
			}
			list, err := endpoints(c)
			if err != nil {
				return err
			}
			if len(list) == 0 {
				return fmt.Errorf("upload requires --upload-url or --github-release")
			}
			pl, err := (&zapp.Project{Upload: list}).Resolve()
			if err != nil {
				return err
			}
			sent, err := pl.UploadFiles(ctx, c.Args().Slice()...)
			for _, u := range sent {
				_, _ = fmt.Fprintf(c.Root().Writer, "%s -> %s\n", u.Artifact, u.URL)
			}
			return err
		},
	}
}
