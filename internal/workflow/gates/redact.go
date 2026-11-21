// Package gates implements deterministic workflow quality gate evaluation,
// secret redaction, and objective-driven verdict calculation.
package gates

import (
	"encoding/json"
	"errors"
	"regexp"
	"sort"
	"strings"
)

// RedactJSON redacts string values in structured evidence without touching
// field names or JSON syntax. It reports whether any value contained a secret.
func RedactJSON(raw string, secrets []string) (string, bool, error) {
	if !json.Valid([]byte(raw)) {
		return "", false, errors.New("invalid JSON artifact")
	}
	var content any
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&content); err != nil {
		return "", false, err
	}
	found := false
	var clean func(any) any
	clean = func(value any) any {
		switch value := value.(type) {
		case string:
			found = HasSecrets(value, secrets) || found
			return Redact(value, secrets)
		case []any:
			for i, item := range value {
				value[i] = clean(item)
			}
		case map[string]any:
			for key, item := range value {
				value[key] = clean(item)
			}
		}
		return value
	}
	content = clean(content)
	if !found {
		return raw, false, nil
	}
	encoded, err := json.Marshal(content)
	return string(encoded), found, err
}

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

type textSpan struct {
	start int
	end   int
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

// Redact replaces all fixed secret patterns and explicit secret values in a single
// merged-span pass over input so overlapping matches never leak suffixes or
// re-scan inserted placeholders.
func Redact(input string, secretValues []string) string {
	if input == "" {
		return input
	}

	var spans []textSpan
	for _, re := range secretPatterns {
		for _, loc := range re.FindAllStringIndex(input, -1) {
			if loc[1] > loc[0] {
				spans = append(spans, textSpan{start: loc[0], end: loc[1]})
			}
		}
	}

	placeholderSpans := findPlaceholderSpans(input)
	for _, secret := range secretValues {
		if strings.TrimSpace(secret) == "" {
			continue
		}
		for offset := 0; offset <= len(input)-len(secret); {
			idx := strings.Index(input[offset:], secret)
			if idx < 0 {
				break
			}
			abs := offset + idx
			sp := textSpan{start: abs, end: abs + len(secret)}
			if !insideAnySpan(sp, placeholderSpans) {
				spans = append(spans, sp)
			}
			offset = abs + 1
		}
	}

	if len(spans) == 0 {
		return input
	}

	sort.Slice(spans, func(i, j int) bool {
		if spans[i].start != spans[j].start {
			return spans[i].start < spans[j].start
		}
		return spans[i].end > spans[j].end
	})

	merged := make([]textSpan, 0, len(spans))
	cur := spans[0]
	for _, sp := range spans[1:] {
		if sp.start < cur.end {
			if sp.end > cur.end {
				cur.end = sp.end
			}
			continue
		}
		merged = append(merged, cur)
		cur = sp
	}
	merged = append(merged, cur)

	var b strings.Builder
	b.Grow(len(input))
	pos := 0
	for _, sp := range merged {
		b.WriteString(input[pos:sp.start])
		b.WriteString(RedactedPlaceholder)
		pos = sp.end
	}
	b.WriteString(input[pos:])
	return b.String()
}

func findPlaceholderSpans(input string) []textSpan {
	var spans []textSpan
	for offset := 0; offset <= len(input)-len(RedactedPlaceholder); {
		idx := strings.Index(input[offset:], RedactedPlaceholder)
		if idx < 0 {
			break
		}
		abs := offset + idx
		end := abs + len(RedactedPlaceholder)
		spans = append(spans, textSpan{start: abs, end: end})
		offset = end
	}
	return spans
}

func insideAnySpan(candidate textSpan, containers []textSpan) bool {
	for _, c := range containers {
		if candidate.start >= c.start && candidate.end <= c.end {
			return true
		}
	}
	return false
}
