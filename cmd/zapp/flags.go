package main

import (
	"github.com/ironpark/zapp"
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
	}, append(certificateFlags(), notaryKeyFlag(), notarizeTimeoutFlag())...)
}

// certificateFlags supply a signing certificate. A PKCS#12 bundle works on every
// host: macOS imports it into a temporary keychain for the run, elsewhere
// rcodesign reads it directly. The secrets also read from the environment, so
// CI can pass them without putting them on the command line.
func certificateFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:  "p12-file",
			Usage: "Path to a PKCS#12 certificate bundle to sign with",
		},
		&cli.StringFlag{
			Name:    "p12-base64",
			Usage:   "PKCS#12 certificate bundle as base64, e.g. from a CI secret (instead of --p12-file)",
			Sources: cli.EnvVars("ZAPP_P12_BASE64"),
		},
		&cli.StringFlag{
			Name:    "p12-password",
			Usage:   "Password for the PKCS#12 bundle",
			Sources: cli.EnvVars("ZAPP_P12_PASSWORD"),
		},
		&cli.StringFlag{
			Name:  "p12-password-file",
			Usage: "File holding the password for the PKCS#12 bundle",
		},
		&cli.StringFlag{
			Name:  "pem-file",
			Usage: "Path to a PEM bundle with the certificate and private key to sign with",
		},
		&cli.StringFlag{
			Name:  "entitlements",
			Usage: "Entitlements plist to sign the app with (default: keep the app's own)",
		},
	}
}

// notarizeTimeoutFlag bounds the wait for Apple's verdict.
func notarizeTimeoutFlag() cli.Flag {
	return &cli.StringFlag{
		Name:  "notarize-timeout",
		Usage: "How long to wait for Apple's notarization verdict, such as 30m or 2h (default: " + zapp.DefaultNotarizeTimeout + ")",
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
