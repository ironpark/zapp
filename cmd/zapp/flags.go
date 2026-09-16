package main

import (
	"github.com/urfave/cli/v3"
)

func subTaskFlags() []cli.Flag {
	return append([]cli.Flag{
		&cli.BoolFlag{
			Category: "[with --notarize (default: false)]",
			Name:     "notarize",
			Hidden:   true,
		},
		&cli.StringFlag{
			Category: "[with --notarize (default: false)]",
			Name:     "profile",
			Aliases:  []string{"p"},
			Usage:    "Keychain profile name",
		},
		&cli.StringFlag{
			Category: "[with --notarize (default: false)]",
			Name:     "apple-id",
			Usage:    "Apple ID email",
		},
		&cli.StringFlag{
			Category: "[with --notarize (default: false)]",
			Name:     "password",
			Usage:    "Apple ID password or app-specific password",
		},
		&cli.StringFlag{
			Category: "[with --notarize (default: false)]",
			Name:     "team-id",
			Usage:    "Developer Team ID",
		},
		&cli.BoolFlag{
			Category: "[with --notarize (default: false)]",
			Name:     "staple",
			Usage:    "Perform stapling after notarization",
		},
		&cli.BoolFlag{
			Category: "[with --sign (default: false)]",
			Name:     "sign",
			Usage:    "Codesign after creating DMG",
			Hidden:   true,
		},
		&cli.StringFlag{
			Category: "[with --sign (default: false)]",
			Name:     "identity",
			Usage:    "Identity to use for signing",
		},
	}, append(certificateFlags(), notaryKeyFlag())...)
}

// certificateFlags name a signing certificate by file. Away from macOS there is
// no keychain to take an identity from, so these are how a certificate is
// supplied. macOS always uses Apple's tools and a keychain identity.
func certificateFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:  "p12-file",
			Usage: "Path to a PKCS#12 certificate bundle to sign with",
		},
		&cli.StringFlag{
			Name:  "p12-password",
			Usage: "Password for the PKCS#12 bundle",
		},
		&cli.StringFlag{
			Name:  "p12-password-file",
			Usage: "File holding the password for the PKCS#12 bundle",
		},
		&cli.StringFlag{
			Name:  "pem-file",
			Usage: "Path to a PEM certificate bundle to sign with",
		},
	}
}

// notaryKeyFlag names an App Store Connect API key, which is how rcodesign
// authenticates to the notary service.
func notaryKeyFlag() cli.Flag {
	return &cli.StringFlag{
		Name:  "api-key-file",
		Usage: "App Store Connect API key JSON to notarize with",
	}
}
