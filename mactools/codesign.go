package mactools

import (
	"context"
	"os/exec"
)

func Codesign(ctx context.Context, keychain, developerID, appPath string) {
	defaultArgs := []string{
		"--force", "--deep", "--sign", developerID, "--options=runtime", appPath,
	}
	if keychain != "" {
		defaultArgs = append([]string{"--keychain", keychain}, defaultArgs...)
	}

	cmd := exec.CommandContext(ctx, "codesign", defaultArgs...)
	cmd.Run()
}
