package otool

import (
	"os/exec"
	"strings"
)

func GetDependencies(file string) ([]string, error) {
	cmd := exec.Command("otool", "-L", file)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return parseOtoolOutput(string(output))
}

func parseOtoolOutput(output string) ([]string, error) {
	lines := strings.Split(output, "\n")
	dependencies := make([]string, 0, len(lines))
	for _, line := range lines {
		// Frameworks are not needed (is system dependent)
		if strings.Contains(line, "/System/Library/Frameworks/") {
			continue
		}
		if strings.Contains(line, "(compatibility version") {
			fields := strings.Fields(line)
			dependencies = append(dependencies, fields[0])
		}
	}
	return dependencies, nil
}
