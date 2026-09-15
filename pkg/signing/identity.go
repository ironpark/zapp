package signing

import (
	"bufio"
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/ironpark/zapp/internal/macexec"
)

type Identity struct {
	ID            int
	Fingerprint   string
	Description   string
	Type          string
	DeveloperName string
	DeveloperID   string
}

// SecureString renders the identity with the developer name fully masked and
// all but the first few characters of the developer ID masked, so it is safe to
// print in logs and CI output.
func (i Identity) SecureString() string {
	// Mask the developer name.
	nameParts := strings.Fields(i.DeveloperName)
	maskedName := make([]string, len(nameParts))
	for j, part := range nameParts {
		maskedName[j] = strings.Repeat("*", len(part))
	}
	securedName := strings.Join(maskedName, " ")

	return fmt.Sprintf("%s: %s (%s)", i.Type, securedName, maskID(i.DeveloperID))
}

// idPrefixLen is how many leading characters of a developer ID stay visible.
const idPrefixLen = 5

// maskID hides all but the leading characters of a developer ID. A description
// that does not match descRegexp leaves DeveloperID empty, and identities can
// carry IDs shorter than the prefix, so the length is never assumed.
func maskID(id string) string {
	if len(id) <= idPrefixLen {
		return strings.Repeat("*", len(id))
	}
	return id[:idPrefixLen] + strings.Repeat("*", len(id)-idPrefixLen)
}
func (i Identity) String() string {
	return fmt.Sprintf("%s: %s (%s)", i.Type, i.DeveloperName, i.DeveloperID)
}

func listIdentities(ctx context.Context, keychain string) ([]Identity, error) {
	args := []string{"find-identity", "-v"}
	if keychain != "" {
		args = append(args, "-k", keychain)
	}
	output, err := macexec.Run(ctx, "security", args...)
	if err != nil {
		return nil, err
	}
	return parseFindIdentityOutput(output)
}

var (
	// lineRegexp Regular expressions for parsing certificate information
	lineRegexp = regexp.MustCompile(`^\s*(\d+)\) ([A-F0-9]+) "([^"]+)"$`)
	// descRegexp Regular expression to separate description string into Type, DeveloperName, DeveloperID
	descRegexp = regexp.MustCompile(`^(.*?):\s(.*?)\s\((.*?)\)$`)
)

func parseFindIdentityOutput(output string) (identities []Identity, err error) {
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		matches := lineRegexp.FindStringSubmatch(line)
		if matches != nil {
			id, err := strconv.Atoi(matches[1])
			if err != nil {
				return nil, err
			}
			desc := matches[3]
			descMatches := descRegexp.FindStringSubmatch(desc)
			if descMatches != nil {
				identities = append(identities, Identity{
					ID:            id,
					Fingerprint:   matches[2],
					Description:   desc,
					Type:          descMatches[1],
					DeveloperName: descMatches[2],
					DeveloperID:   descMatches[3],
				})
			} else {
				identities = append(identities, Identity{
					ID: id, Fingerprint: matches[2], Description: desc,
				})
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return identities, nil
}
