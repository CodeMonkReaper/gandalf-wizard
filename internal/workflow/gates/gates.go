package gates

import (
	"encoding/json"
	"io"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/CodeMonkReaper/gandalf-wizard/internal/workflow/schema"
)

const (
	GateStatusPass             = "pass"
	GateStatusFail             = "fail"
	GateStatusBlocked          = "blocked"
	GateStatusNeedsHumanReview = "needs_human_review"
	// GateStatusSkipped records a gate that was never evaluated because its
	// optional branch was left undecided, so it is not mistaken for a failure.
	GateStatusSkipped = "skipped"
)

// GateResult summarizes the deterministic outcome of evaluating a quality gate.
type GateResult struct {
	Status      string   `json:"status"`
	Retryable   bool     `json:"retryable"`
	Informative bool     `json:"informative"`
	Violations  []string `json:"violations,omitempty"`
	Warnings    []string `json:"warnings,omitempty"`
}

// EvaluateGate evaluates a closed catalog of deterministic rules over raw step output.
func EvaluateGate(spec schema.GateSpec, rawOutput string, secretValues []string) GateResult {
	return EvaluateGateWithSecretEvidence(spec, rawOutput, HasSecrets(rawOutput, secretValues))
}

// EvaluateGateWithSecretEvidence accepts detection from the decoded values
// when the output has a structured representation.
func EvaluateGateWithSecretEvidence(spec schema.GateSpec, rawOutput string, hasSecrets bool) GateResult {
	if len(spec.Rules) == 0 {
		res := GateResult{
			Status:      GateStatusFail,
			Retryable:   !spec.Informative,
			Informative: spec.Informative,
			Violations:  []string{"no_rules: gate spec declares no rules"},
		}
		if spec.Informative {
			res.Warnings = append([]string(nil), res.Violations...)
		}
		return res
	}

	// Pass 1: no_secrets blocks immediately, before any other rule runs.
	for _, rule := range spec.Rules {
		if rule.Kind == schema.RuleNoSecrets && hasSecrets {
			return GateResult{
				Status:      GateStatusBlocked,
				Retryable:   false,
				Informative: false,
				Violations:  []string{"no_secrets: secret material detected in output"},
			}
		}
	}

	violations := []string{}
	for _, rule := range spec.Rules {
		switch rule.Kind {
		case schema.RuleNoSecrets:
			// Already handled in pass 1; a clean output satisfies it.
		case schema.RuleNonEmpty:
			if strings.TrimSpace(rawOutput) == "" {
				violations = append(violations, "non_empty: output is empty or whitespace only")
			}
		case schema.RuleMinLength:
			if rule.Limit < 1 {
				violations = append(violations, "min_length: invalid limit, must be >= 1")
				continue
			}
			if utf8.RuneCountInString(rawOutput) < rule.Limit {
				violations = append(violations, "min_length: output shorter than required rune count")
			}
		case schema.RuleMaxLength:
			if rule.Limit < 1 {
				violations = append(violations, "max_length: invalid limit, must be >= 1")
				continue
			}
			if utf8.RuneCountInString(rawOutput) > rule.Limit {
				violations = append(violations, "max_length: output longer than allowed rune count")
			}
		case schema.RuleContains:
			if rule.Value == "" {
				violations = append(violations, "contains: empty required value")
				continue
			}
			if !strings.Contains(rawOutput, rule.Value) {
				violations = append(violations, "contains: required substring not found")
			}
		case schema.RuleNotContains:
			if rule.Value == "" {
				violations = append(violations, "not_contains: empty forbidden value")
				continue
			}
			if strings.Contains(rawOutput, rule.Value) {
				violations = append(violations, "not_contains: forbidden substring found")
			}
		case schema.RuleRegexMatch:
			re, ok := compileRuleRegex("regex_match", rule.Value, &violations)
			if !ok {
				continue
			}
			if !re.MatchString(rawOutput) {
				violations = append(violations, "regex_match: output does not match required pattern")
			}
		case schema.RuleRegexForbid:
			re, ok := compileRuleRegex("regex_forbid", rule.Value, &violations)
			if !ok {
				continue
			}
			if re.MatchString(rawOutput) {
				violations = append(violations, "regex_forbid: output matches forbidden pattern")
			}
		case schema.RuleJSONValid:
			if !json.Valid([]byte(rawOutput)) {
				violations = append(violations, "json_valid: output is not valid JSON")
			}
		case schema.RuleJSONRequiredKeys:
			checkJSONRequiredKeys(rule, rawOutput, &violations)
		default:
			violations = append(violations, "unknown_rule: unrecognized rule kind fails closed")
		}
	}

	if len(violations) > 0 {
		result := GateResult{
			Status:      GateStatusFail,
			Retryable:   !spec.Informative,
			Informative: spec.Informative,
			Violations:  violations,
		}
		if spec.Informative {
			result.Warnings = append([]string(nil), violations...)
		}
		return result
	}

	return GateResult{
		Status:      GateStatusPass,
		Informative: spec.Informative,
	}
}

func compileRuleRegex(kind, pattern string, violations *[]string) (*regexp.Regexp, bool) {
	if pattern == "" {
		*violations = append(*violations, kind+": empty pattern fails closed")
		return nil, false
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		*violations = append(*violations, kind+": invalid pattern fails closed")
		return nil, false
	}
	return re, true
}

func checkJSONRequiredKeys(rule schema.GateRuleSpec, rawOutput string, violations *[]string) {
	if len(rule.Keys) == 0 {
		*violations = append(*violations, "json_required_keys: no keys configured fails closed")
		return
	}
	for _, key := range rule.Keys {
		if strings.TrimSpace(key) == "" {
			*violations = append(*violations, "json_required_keys: empty key name fails closed")
			return
		}
	}

	decoder := json.NewDecoder(strings.NewReader(rawOutput))
	var obj map[string]any
	if err := decoder.Decode(&obj); err != nil {
		*violations = append(*violations, "json_required_keys: output is not a JSON object")
		return
	}
	if obj == nil {
		*violations = append(*violations, "json_required_keys: output is JSON null")
		return
	}
	// A second decode must hit EOF; anything else means trailing garbage.
	if _, err := decoder.Token(); err != io.EOF {
		*violations = append(*violations, "json_required_keys: trailing data after JSON object")
		return
	}

	missing := []string{}
	for _, key := range rule.Keys {
		if _, present := obj[key]; !present {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		*violations = append(*violations, "json_required_keys: missing keys: "+strings.Join(missing, ", "))
	}
}
