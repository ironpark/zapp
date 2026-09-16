package main

import (
	"context"
	_ "embed"
	"encoding/csv"
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/urfave/cli/v3"
)

//go:embed 3rdparty.csv
var thirdPartyLicensesCsv string

var Version string = "v0.2.4"
var BuildDate string = "2024-10-16"
var Commit string = "50261600d655ae526b7645d05d2bc573e3a8dee5"

var infoCommand = &cli.Command{
	Name:   "info",
	Usage:  "Display detailed information about the current ZAPP build",
	Action: infoAction,
}

func infoAction(ctx context.Context, c *cli.Command) error {
	if err := printInfo(c.Root()); err != nil {
		return err
	}
	err := print3rdPartyLicenseOverview(c.Root())
	if err != nil {
		return err
	}
	return nil
}

func printInfo(app *cli.Command) error {
	c0 := color.New(color.FgCyan, color.Bold)
	c2 := color.New(color.FgHiWhite)
	if _, err := c0.Fprintln(app.Writer, "[Build Info]"); err != nil {
		return err
	}
	if _, err := c2.Fprintf(app.Writer, "%-12s: %s\n", "Name", color.GreenString("ZAPP")); err != nil {
		return err
	}
	if _, err := c2.Fprintf(app.Writer, "%-12s: %s\n", "Version", color.GreenString(Version)); err != nil {
		return err
	}
	if _, err := c2.Fprintf(app.Writer, "%-12s: %s\n", "Build Date", color.GreenString(BuildDate)); err != nil {
		return err
	}
	if _, err := c2.Fprintf(app.Writer, "%-12s: %s\n\n", "Commit Hash", color.GreenString(Commit)); err != nil {
		return err
	}

	if _, err := c0.Fprintln(app.Writer, "[License]"); err != nil {
		return err
	}
	if _, err := c2.Fprintf(app.Writer, "%-12s: %s\n", "Type", "MIT License"); err != nil {
		return err
	}
	if _, err := c2.Fprintf(app.Writer, "%-12s: %s\n\n", "URL", "https://raw.githubusercontent.com/ironpark/zapp/refs/heads/main/LICENSE"); err != nil {
		return err
	}
	return nil
}

func print3rdPartyLicenseOverview(app *cli.Command) error {
	c0 := color.New(color.FgCyan, color.Bold)
	c1 := color.New(color.FgGreen, color.Italic)
	c2 := color.New(color.FgHiWhite)

	// Get csv data (fossa scan result)
	thirdPartyLicenses := strings.Split(thirdPartyLicensesCsv, "    Direct Dependencies\n")
	// Skip the first 4 lines (First Party Licenses header)
	thirdPartyLicenses = thirdPartyLicenses[1:]

	// Parse CSV (3rd party licenses)
	reader := csv.NewReader(strings.NewReader(strings.Join(thirdPartyLicenses, "\n")))
	records, err := reader.ReadAll()
	if err != nil {
		return err
	}
	if len(records) == 0 {
		return nil
	}
	getUrl := func(name, downloadLink string) string {
		if downloadLink == "" && strings.HasPrefix(name, "github.com/") {
			return "https://" + name
		}
		if strings.HasPrefix(downloadLink, "https://github.com/") {
			return strings.Split(downloadLink, "/archive/")[0]
		}
		return downloadLink
	}
	if _, err := c0.Fprintln(app.Writer, "[Included 3rdParty libraries]"); err != nil {
		return err
	}
	if _, err := c2.Fprintf(app.Writer, "%-28s %-12s %-35s %s\n", "Name", "Commit", "License", "URL"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(app.Writer, "---------------------------------------------------------------------------------------------------------------"); err != nil {
		return err
	}
	for _, record := range records[1:] {
		name := strings.TrimSpace(record[0])
		url := getUrl(name, strings.TrimSpace(record[len(record)-2]))
		recordStr := c1.Sprintf("%-28s ", name)
		recordStr += fmt.Sprintf("%-12s ", record[1][:12])
		recordStr += c2.Sprintf("%-35s", record[3])
		if url != "" {
			recordStr += fmt.Sprintf(" %s", url)
		}
		if _, err := fmt.Fprintln(app.Writer, recordStr); err != nil {
			return err
		}
	}
	return nil
}
