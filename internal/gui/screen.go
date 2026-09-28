package gui

// Help describes the actual configuration workflow for each step.
var stepHelp = map[int][][2]string{
	tabProject: {
		{"APP BUNDLE", "Select the .app directory. Its Info.plist supplies default package identity and version."},
		{"OUTPUT", "Relative paths use the project directory. A new output directory is created during the build."},
		{"SAVE & BUILD", "Save keeps your configuration. Validate checks inputs. Build uses current settings, including unsaved edits."},
	},
	tabPKG: {
		{"SINGLE APP", "Leave identity and version blank to read Info.plist. The default destination is /Applications."},
		{"PACKAGE TYPE", "Default produces a product installer. Component produces a single component package."},
		{"COMPONENT PAYLOAD", "Root is the source directory. Entry selects one direct child; blank includes all contents. A blank install location uses /."},
	},
	tabDep: {
		{"SEARCH DIRECTORIES", "Enter one directory per line. Spaces within a path are preserved; empty lines are ignored."},
		{"AUTOMATIC DISCOVERY", "Leave the list blank to use the dependency resolver's automatic search. Paths may be relative to this project."},
		{"BUILD ORDER", "Libraries are bundled before signing and packaging. Disable this step if your app has no external dependencies."},
	},
	tabSign: {
		{"macOS", "Use a Keychain signing identity, or a PKCS#12 or PEM certificate, which is imported into a temporary keychain for the build."},
		{"WINDOWS & LINUX", "Use a PKCS#12 or PEM certificate with rcodesign. Supply a password file when the certificate requires it."},
		{"CREDENTIALS", "Store password file paths here. Certificate passwords are not saved in the project."},
	},
	tabNotarize: {
		{"macOS", "Use a saved notarytool Keychain profile. Apple ID authentication additionally requires a runtime password and Team ID."},
		{"WINDOWS & LINUX", "Use the rcodesign API key JSON file. Keychain profiles and Apple ID credentials are macOS-only."},
		{"STAPLING", "Enable Staple to attach the approved notarization ticket for offline verification."},
	},
}
