// Package gates implements deterministic workflow quality gate evaluation,
// secret redaction, and objective-driven verdict calculation.
package gates

import (
	"regexp"
	"sort"
	"strings"
)

// RedactedPlaceholder replaces detected sensitive material in outputs and logs.
const RedactedPlaceholder = "[redactado]"

// Secret patterns evaluated over raw output. The full PEM block is matched
// before the bare header so the private key body is redacted too.
var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?s)-----BEGIN (?:RSA |EC |OPENSSH |)?PRIVATE KEY-----.*?-----END (?:RSA |EC |OPENSSH |)?PRIVATE KEY-----`),
	regexp.MustCompile(`-----BEGIN (?:RSA |EC |OPENSSH |)PRIVATE KEY-----`),
	regexp.MustCompile(`mstrl_[A-Za-z0-9_-]{12,}`),
	regexp.MustCompile(`sk-(?:proj-|live-|ant-|)[A-Za-z0-9_-]{24,}`),
	regexp.MustCompile(`AIza[0-9A-Za-z_-]{32,}`),
	regexp.MustCompile(`(?:gh[pousr]_[A-Za-z0-9]{30,}|github_pat_[A-Za-z0-9_]{30,})`),
}

// HasSecrets reports whether input contains any known secret pattern or explicit secret value.
func HasSecrets(input string, secretValues []string) bool {
	for _, re := range secretPatterns {
		if re.MatchString(input) {
			return true
		}
	}
	for _, secret := range secretValues {
		if strings.TrimSpace(secret) == "" {
			continue
		}
		if strings.Contains(input, secret) {
			return true
		}
	}
	return false
}

// Redact replaces any known secret pattern or explicit secret value in input with RedactedPlaceholder.
func Redact(input string, secretValues []string) string {
	out := input
	sorted := make([]string, 0, len(secretValues))
	for _, secret := range secretValues {
		if strings.TrimSpace(secret) == "" {
			continue
		}
		sorted = append(sorted, secret)
	}
	sort.SliceStable(sorted, func(i, j int) bool {
		return len(sorted[i]) > len(sorted[j])
	})
	for _, secret := range sorted {
		out = strings.ReplaceAll(out, secret, RedactedPlaceholder)
	}
	for _, re := range secretPatterns {
		out = re.ReplaceAllString(out, RedactedPlaceholder)
	}
	return out
}
