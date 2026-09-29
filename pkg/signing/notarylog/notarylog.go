// Package notarylog reads the log Apple's notary service keeps for each
// submission, which says why one was rejected.
package notarylog

import (
	"encoding/json"
	"fmt"
	"strings"
)

type log struct {
	Status        string  `json:"status"`
	StatusSummary string  `json:"statusSummary"`
	Issues        []issue `json:"issues"`
}

type issue struct {
	Severity     string `json:"severity"`
	Path         string `json:"path"`
	Message      string `json:"message"`
	DocURL       string `json:"docUrl"`
	Architecture string `json:"architecture"`
}

// Summary renders a log's verdict and its issues, one per file and message,
// with the architectures it was found in. An issue's path starts with the
// archive submitted, which is left out. It returns "" for anything that is
// not a notary log.
func Summary(data []byte) string {
	var l log
	if err := json.Unmarshal(data, &l); err != nil || (l.Status == "" && l.StatusSummary == "" && len(l.Issues) == 0) {
		return ""
	}
	type key struct{ severity, path, message, doc string }
	var order []key
	archs := map[key][]string{}
	for _, i := range l.Issues {
		k := key{i.Severity, i.Path, i.Message, i.DocURL}
		if _, ok := archs[k]; !ok {
			order = append(order, k)
		}
		if i.Architecture != "" {
			archs[k] = append(archs[k], i.Architecture)
		} else if archs[k] == nil {
			archs[k] = []string{}
		}
	}
	var b strings.Builder
	b.WriteString(strings.TrimSpace(l.StatusSummary))
	if b.Len() == 0 {
		b.WriteString(l.Status)
	}
	for _, k := range order {
		path := k.path
		if _, rest, ok := strings.Cut(path, "/"); ok {
			path = rest
		}
		where := path
		if a := archs[k]; len(a) > 0 {
			where += " (" + strings.Join(a, ", ") + ")"
		}
		severity := ""
		if k.severity != "" && k.severity != "error" {
			severity = k.severity + ": "
		}
		fmt.Fprintf(&b, "\n  %s%s: %s", severity, where, k.message)
		if k.doc != "" {
			b.WriteString("\n    " + k.doc)
		}
	}
	return b.String()
}
