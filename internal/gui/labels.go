package gui

// Labels the editor finds fields by as well as shows. Validation moves the
// user to the field a problem names, and a method switch to the fields its
// method owns, so each such label is one constant shared by the field and
// every lookup.
const (
	labelAppBundle        = "App bundle"
	labelPackageType      = "Package type"
	labelComponents       = "Components"
	labelComponentID      = "Component ID"
	labelRootDirectory    = "Root directory"
	labelScriptsDirectory = "Scripts directory"
	labelDistribution     = "Distribution"
	labelItemIcon         = "Item icon"

	labelSigningIdentity = "Signing identity"
	labelP12Certificate  = "PKCS#12 certificate"
	labelPasswordFile    = "Password file"
	labelPEMCertificate  = "PEM certificate"

	labelKeychainProfile = "Keychain profile"
	labelAppleID         = "Apple ID"
	labelTeamID          = "Team ID"
	labelAppPassword     = "App-specific password"
	labelAPIKeyFile      = "API key file"
	labelNotaryTimeout   = "Timeout"

	labelZIPOutput       = "ZIP output"
	labelChecksumsOutput = "Checksums output"
	labelUploads         = "Uploads"
	labelAppcast         = "Sparkle appcast"
	labelHomebrew        = "Homebrew cask"
)
