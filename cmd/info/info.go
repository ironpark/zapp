package info

import (
	"encoding/csv"
	"fmt"
	"github.com/fatih/color"
	"github.com/urfave/cli/v2"
	"strings"
)
import _ "embed"

//go:embed 3rdparty.csv
var thirdPartyLicensesCsv string

var Version string = "v0.2.4"
var BuildDate string = "2024-10-16"
var Commit string = "50261600d655ae526b7645d05d2bc573e3a8dee5"

var Command = &cli.Command{
	Name:   "info",
	Usage:  "Display detailed information about the current ZAPP build",
	Args:   true,
	Action: action,
}

func action(c *cli.Context) error {
	printInfo(c.App)
	err := print3rdPartyLicenseOverview(c.App)
	if err != nil {
		return err
	}
	return nil
}

func printInfo(app *cli.App) {
	c0 := color.New(color.FgCyan, color.Bold)
	c2 := color.New(color.FgHiWhite)
	c0.Fprintln(app.Writer, "[Build Info]")
	c2.Printf("%-12s: %s\n", "Name", color.GreenString("ZAPP"))
	c2.Printf("%-12s: %s\n", "Version", color.GreenString(Version))
	c2.Printf("%-12s: %s\n", "Build Date", color.GreenString(BuildDate))
	c2.Printf("%-12s: %s\n\n", "Commit Hash", color.GreenString(Commit))

	c0.Fprintln(app.Writer, "[License]")
	c2.Printf("%-12s: %s\n", "Type", "MIT License")
	c2.Printf("%-12s: %s\n\n", "URL", "https://raw.githubusercontent.com/ironpark/zapp/refs/heads/main/LICENSE")
}

func print3rdPartyLicenseOverview(app *cli.App) error {
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
	c0.Fprintln(app.Writer, "[Included 3rdParty libraries]")
	c2.Printf("%-28s %-12s %-35s %s\n", "Name", "Commit", "License", "URL")
	fmt.Println("---------------------------------------------------------------------------------------------------------------")
	for _, record := range records[1:] {
		name := strings.TrimSpace(record[0])
		url := getUrl(name, strings.TrimSpace(record[len(record)-2]))
		recordStr := c1.Sprintf("%-28s ", name)
		recordStr += fmt.Sprintf("%-12s ", record[1][:12])
		recordStr += c2.Sprintf("%-35s", record[3])
		if url != "" {
			recordStr += fmt.Sprintf(" %s", url)
		}
		fmt.Println(recordStr)
	}
	return nil
}
