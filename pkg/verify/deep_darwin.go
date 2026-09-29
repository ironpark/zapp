package verify

import (
	"context"
	"os/exec"
	"strings"
)

// deepApp runs Apple's own checks: codesign for the signatures and sealed
// resources of everything in the bundle, spctl for what Gatekeeper decides.
func deepApp(ctx context.Context, r *Report, path string) {
	tool(ctx, r, "signature integrity", "codesign", "--verify", "--deep", "--strict", "--verbose=2", path)
	tool(ctx, r, "Gatekeeper", "spctl", "--assess", "--type", "execute", "--verbose=2", path)
}

func deepDMG(ctx context.Context, r *Report, path string) {
	tool(ctx, r, "signature integrity", "codesign", "--verify", "--strict", "--verbose=2", path)
	tool(ctx, r, "Gatekeeper", "spctl", "--assess", "--type", "open", "--context", "context:primary-signature", "--verbose=2", path)
}

func deepPKG(ctx context.Context, r *Report, path string) {
	tool(ctx, r, "signature integrity", "pkgutil", "--check-signature", path)
	tool(ctx, r, "Gatekeeper", "spctl", "--assess", "--type", "install", "--verbose=2", path)
}

// tool adds a check passed when the command succeeds, with what it said:
// these tools report on stderr.
func tool(ctx context.Context, r *Report, name, command string, args ...string) {
	out, err := exec.CommandContext(ctx, command, args...).CombinedOutput()
	detail := summarize(strings.ReplaceAll(string(out), args[len(args)-1]+": ", ""))
	if err != nil {
		if detail == "" {
			detail = err.Error()
		}
		r.add(name, Fail, "%s", detail)
		return
	}
	r.add(name, Pass, "%s", detail)
}

// summarize keeps a tool's first few lines, without the path it repeats.
func summarize(out string) string {
	var lines []string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		// codesign --deep lists each nested bundle it prepares and validates.
		if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "--") {
			lines = append(lines, line)
		}
	}
	if len(lines) > 4 {
		lines = append(lines[:4], "…")
	}
	var b strings.Builder
	for i, line := range lines {
		if i > 0 {
			if strings.HasSuffix(lines[i-1], ":") {
				b.WriteString(" ")
			} else {
				b.WriteString("; ")
			}
		}
		b.WriteString(line)
	}
	return b.String()
}
