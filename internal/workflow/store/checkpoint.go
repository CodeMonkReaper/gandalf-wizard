package store

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"
)

const redactionPlaceholder = "[redactado]"

// Secret patterns compiled locally to keep internal/workflow/store decoupled
// from internal/workflow/gates while providing defense-in-depth redaction.
var redactionPatterns = []*regexp.Regexp{
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

// redactText replaces all fixed secret patterns and explicit secret values in
// one non-overlapping pass over the original text so overlapping matches never
// leak suffixes or re-scan previously inserted placeholders.
func redactText(text string, secrets []string) string {
	if text == "" {
		return text
	}
	var spans []textSpan
	for _, re := range redactionPatterns {
		for _, loc := range re.FindAllStringIndex(text, -1) {
			if loc[1] > loc[0] {
				spans = append(spans, textSpan{start: loc[0], end: loc[1]})
			}
		}
	}
	placeholderSpans := findPlaceholderSpans(text)
	for _, secret := range secrets {
		if strings.TrimSpace(secret) == "" {
			continue
		}
		for offset := 0; offset <= len(text)-len(secret); {
			idx := strings.Index(text[offset:], secret)
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
		return text
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
	b.Grow(len(text))
	pos := 0
	for _, sp := range merged {
		b.WriteString(text[pos:sp.start])
		b.WriteString(redactionPlaceholder)
		pos = sp.end
	}
	b.WriteString(text[pos:])
	return b.String()
}

func findPlaceholderSpans(input string) []textSpan {
	var spans []textSpan
	for offset := 0; offset <= len(input)-len(redactionPlaceholder); {
		idx := strings.Index(input[offset:], redactionPlaceholder)
		if idx < 0 {
			break
		}
		abs := offset + idx
		end := abs + len(redactionPlaceholder)
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

func redactStringSlice(values []string, secrets []string) []string {
	if values == nil {
		return nil
	}
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = redactText(v, secrets)
	}
	return out
}

func redactArtifacts(artifacts []ArtifactRef, secrets []string) []ArtifactRef {
	if artifacts == nil {
		return nil
	}
	out := make([]ArtifactRef, len(artifacts))
	for i, a := range artifacts {
		a.Summary = redactText(a.Summary, secrets)
		out[i] = a
	}
	return out
}

func redactRunState(state *RunState, secrets []string) {
	state.Error = redactText(state.Error, secrets)
	state.Warnings = redactStringSlice(state.Warnings, secrets)
	for id, step := range state.Steps {
		step.Error = redactText(step.Error, secrets)
		step.Warnings = redactStringSlice(step.Warnings, secrets)
		step.Artifacts = redactArtifacts(step.Artifacts, secrets)
		for i := range step.Attempts {
			step.Attempts[i].Diagnosis = redactText(step.Attempts[i].Diagnosis, secrets)
			step.Attempts[i].Error = redactText(step.Attempts[i].Error, secrets)
			step.Attempts[i].Artifacts = redactArtifacts(step.Attempts[i].Artifacts, secrets)
		}
		state.Steps[id] = step
	}
	for id, gate := range state.Gates {
		gate.Prompt = redactText(gate.Prompt, secrets)
		gate.Violations = redactStringSlice(gate.Violations, secrets)
		gate.Warnings = redactStringSlice(gate.Warnings, secrets)
		state.Gates[id] = gate
	}
	for i := range state.Approvals {
		state.Approvals[i].Reason = redactText(state.Approvals[i].Reason, secrets)
		state.Approvals[i].DecidedBy = redactText(state.Approvals[i].DecidedBy, secrets)
	}
	if state.ObjectiveVerdict != nil {
		state.ObjectiveVerdict.Warnings = redactStringSlice(state.ObjectiveVerdict.Warnings, secrets)
	}
}

func canonicalizeState(state *RunState) (*RunState, error) {
	data, err := json.Marshal(state)
	if err != nil {
		return nil, err
	}
	var clone RunState
	if err := json.Unmarshal(data, &clone); err != nil {
		return nil, err
	}
	return &clone, nil
}

// ComputeCheckpointDigest returns the canonical SHA-256 hex digest of a RunState snapshot.
func ComputeCheckpointDigest(state *RunState) string {
	if state == nil {
		return ""
	}
	clone, err := canonicalizeState(state)
	if err != nil {
		return ""
	}
	clone.CheckpointDigest = ""
	canonical, err := json.Marshal(clone)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:])
}

// WriteCheckpoint increments state revision, redacts text diagnostics, recomputes
// checkpoint_digest, and atomically replaces state.json within the confined store.
func (s *WorkflowStore) WriteCheckpoint(runID string, state *RunState) (string, error) {
	if s == nil {
		return "", errors.New("workflow store is nil")
	}
	if state == nil {
		return "", errors.New("run state is nil")
	}
	if err := s.ensureRunDir(runID); err != nil {
		return "", err
	}

	secrets := s.snapshotSecrets()
	if state.Steps == nil {
		state.Steps = make(map[string]StepRecord)
	}
	if state.RunID != "" && state.RunID != runID {
		return "", fmt.Errorf("%w: run id mismatch", ErrCorruptState)
	}
	state.RunID = runID
	if state.StateRevision < 1 {
		state.StateRevision = 1
	} else {
		state.StateRevision++
	}
	state.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	redactRunState(state, secrets)
	state.CheckpointDigest = ComputeCheckpointDigest(state)

	data, err := marshalStateJSON(state)
	if err != nil {
		return "", err
	}

	stateRel := path.Join("runs", runID, "state.json")
	if err := s.writeAtomic(stateRel, data); err != nil {
		return "", err
	}
	return state.CheckpointDigest, nil
}

func marshalStateJSON(state *RunState) ([]byte, error) {
	encoded, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal run state: %w", err)
	}
	return append(encoded, '\n'), nil
}

func readStateFile(root *os.Root, stateRel, expectedRunID string) (*RunState, error) {
	raw, err := root.ReadFile(stateRel)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCorruptState, err)
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return nil, fmt.Errorf("%w: empty state file", ErrCorruptState)
	}
	var st RunState
	if err := json.Unmarshal(raw, &st); err != nil {
		return nil, fmt.Errorf("%w: invalid json: %v", ErrCorruptState, err)
	}
	if st.RunID != expectedRunID || st.StateRevision < 1 || st.Steps == nil || len(st.CheckpointDigest) != 64 {
		return nil, fmt.Errorf("%w: missing required fields", ErrCorruptState)
	}
	if ComputeCheckpointDigest(&st) != st.CheckpointDigest {
		return nil, fmt.Errorf("%w: checkpoint digest mismatch", ErrCorruptState)
	}
	return &st, nil
}
