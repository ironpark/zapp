package pkg

import (
	"fmt"
	"strings"
)

type DistributionBuilder struct {
	Title        string
	Organization string
	Identifier   string
	Version      string
	MinOSVersion string
	LicenseFile  string
	Choices      []Choice
}

type Choice struct {
	ID       string
	Visible  bool
	PkgRefID string
}

func NewDistributionBuilder() *DistributionBuilder {
	return &DistributionBuilder{
		MinOSVersion: "10.9",
		Choices:      []Choice{},
	}
}

func (b *DistributionBuilder) AddChoice(id string, visible bool, pkgRefID string) {
	b.Choices = append(b.Choices, Choice{
		ID:       id,
		Visible:  visible,
		PkgRefID: pkgRefID,
	})
}

func (b *DistributionBuilder) AddLicense(file string) {
	b.LicenseFile = file
}

func (b *DistributionBuilder) Build() string {
	var sb strings.Builder

	sb.WriteString(`<?xml version="1.0" encoding="utf-8"?>
<installer-gui-script minSpecVersion="1">
`)
	sb.WriteString(fmt.Sprintf("    <title>%s</title>\n", b.Title))
	sb.WriteString(fmt.Sprintf("    <organization>%s</organization>\n", b.Organization))
	sb.WriteString(`    <domains enable_localSystem="true"/>
    <options customize="never" require-scripts="true" allow-external-scripts="no"/>
`)

	if b.LicenseFile != "" {
		sb.WriteString(fmt.Sprintf("    <license file=\"%s\" mime-type=\"text/plain\"/>\n", b.LicenseFile))
	}

	sb.WriteString(fmt.Sprintf(`    <volume-check>
        <allowed-os-versions>
            <os-version min="%s"/>
        </allowed-os-versions>
    </volume-check>
`, b.MinOSVersion))

	sb.WriteString("    <choices-outline>\n")
	sb.WriteString("        <line choice=\"default\">\n")
	for _, choice := range b.Choices {
		sb.WriteString(fmt.Sprintf("            <line choice=\"%s\"/>\n", choice.ID))
	}
	sb.WriteString("        </line>\n")
	sb.WriteString("    </choices-outline>\n")

	sb.WriteString("    <choice id=\"default\"/>\n")
	for _, choice := range b.Choices {
		sb.WriteString(fmt.Sprintf("    <choice id=\"%s\" visible=\"%t\">\n", choice.ID, choice.Visible))
		sb.WriteString(fmt.Sprintf("        <pkg-ref id=\"%s\"/>\n", choice.PkgRefID))
		sb.WriteString("    </choice>\n")
	}

	for _, choice := range b.Choices {
		sb.WriteString(fmt.Sprintf("    <pkg-ref id=\"%s\" version=\"%s\" onConclusion=\"none\">component.pkg</pkg-ref>\n", choice.PkgRefID, b.Version))
	}

	sb.WriteString("</installer-gui-script>")

	return sb.String()
}
