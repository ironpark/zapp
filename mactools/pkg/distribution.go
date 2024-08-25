package pkg

import (
	"fmt"
	"strings"
)

// DistributionBuilder is used to build the distribution XML for an installer.
type DistributionBuilder struct {
	Title        string   // Title of the installer.
	Organization string   // Organization name.
	Identifier   string   // Unique identifier for the installer.
	Version      string   // Version of the installer.
	MinOSVersion string   // Minimum OS version required for the installer.
	LicenseFile  string   // Path to the license file.
	Choices      []Choice // List of choices available in the installer.
}

// Choice represents an individual choice in the installer.
type Choice struct {
	ID       string // Unique identifier for the choice.
	Visible  bool   // Visibility of the choice in the installer UI.
	PkgRefID string // Package reference ID associated with the choice.
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

	sb.WriteString(`<?xml version="1.0" encoding="utf-8"?>` + "\n")
	sb.WriteString(`<installer-gui-script minSpecVersion="1">`)
	sb.WriteString(fmt.Sprintf("<title>%s</title>", b.Title))
	sb.WriteString(fmt.Sprintf("<organization>%s</organization>", b.Organization))
	sb.WriteString(`<domains enable_localSystem="true"/>`)
	sb.WriteString(`<options customize="never" require-scripts="true" allow-external-scripts="no"/>`)

	if b.LicenseFile != "" {
		sb.WriteString(fmt.Sprintf("<license file=\"%s\" mime-type=\"text/plain\"/>", b.LicenseFile))
	}

	sb.WriteString(fmt.Sprintf(`<volume-check><allowed-os-versions><os-version min="%s"/></allowed-os-versions></volume-check>`, b.MinOSVersion))

	sb.WriteString("<choices-outline>")
	sb.WriteString("<line choice=\"default\">")
	for _, choice := range b.Choices {
		sb.WriteString(fmt.Sprintf("<line choice=\"%s\"/>", choice.ID))
	}
	sb.WriteString("</line>")
	sb.WriteString("</choices-outline>")

	sb.WriteString("<choice id=\"default\"/>")
	for _, choice := range b.Choices {
		sb.WriteString(fmt.Sprintf("<choice id=\"%s\" visible=\"%t\">", choice.ID, choice.Visible))
		sb.WriteString(fmt.Sprintf("<pkg-ref id=\"%s\"/>", choice.PkgRefID))
		sb.WriteString("</choice>")
	}

	for _, choice := range b.Choices {
		sb.WriteString(fmt.Sprintf("<pkg-ref id=\"%s\" version=\"%s\" onConclusion=\"none\">component.pkg</pkg-ref>", choice.PkgRefID, b.Version))
	}

	sb.WriteString("</installer-gui-script>")

	return sb.String()
}
