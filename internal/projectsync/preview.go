// Package projectsync plans, applies, and restores reproducible local
// synchronization of skills, rules, and pattern references.
package projectsync

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/CodeMonkReaper/gandalf-wizard/internal/synccatalog"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/syncfs"
)

const (
	// OwnerGandalf identifies files or shared blocks managed by Gandalf.
	OwnerGandalf = "gandalf"

	// BlockStart and BlockEnd delimit the Gandalf-managed rules block in AGENTS.md.
	BlockStart = "<!-- gandalf:sync:start -->"
	BlockEnd   = "<!-- gandalf:sync:end -->"
)

const (
	ActionCreate = "create"
	ActionUpdate = "update"
)

// NativeProfile describes a verified harness target projection from T001.
type NativeProfile struct {
	Target        string `json:"target"`
	Version       string `json:"version"`
	OS            string `json:"os"`
	Trust         string `json:"trust"`
	SkillLocation string `json:"skill_location"`
	RuleLocation  string `json:"rule_location"`
	Precedence    string `json:"precedence"`
	Verified      bool   `json:"verified"`
}

// DefaultVerifiedProfiles returns the T001-verified Windows profiles for Codex and Vibe.
func DefaultVerifiedProfiles() []NativeProfile {
	return []NativeProfile{
		{
			Target:        "codex",
			Version:       "0.169.0",
			OS:            "windows/amd64",
			Trust:         "trusted",
			SkillLocation: ".agents/skills",
			RuleLocation:  "AGENTS.md",
			Precedence:    "AGENTS.override.md overrides AGENTS.md; .agents/skills discovered",
			Verified:      true,
		},
		{
			Target:        "vibe",
			Version:       "3.1.3",
			OS:            "windows/amd64",
			Trust:         "trusted",
			SkillLocation: ".agents/skills",
			RuleLocation:  "AGENTS.md",
			Precedence:    ".vibe/skills overrides .agents/skills; AGENTS.md discovered",
			Verified:      true,
		},
	}
}

// Receipt records content previously managed and written by Gandalf.
type Receipt struct {
	Path        string `json:"path"`
	Owner       string `json:"owner"`
	AfterSHA256 string `json:"after_sha256"`
}

// Change describes one planned file mutation. Bytes remain internal to planner/writer.
type Change struct {
	Path    string           `json:"path"`
	Action  string           `json:"action"`
	Targets []string         `json:"targets"`
	Before  syncfs.FileState `json:"-"`
	After   syncfs.FileState `json:"-"`
	Owner   string           `json:"owner"`
}

// Diagnostic describes a typed preview issue without leaking file contents.
type Diagnostic struct {
	Code    string `json:"code"`
	Target  string `json:"target,omitempty"`
	Path    string `json:"path,omitempty"`
	Message string `json:"message"`
}

// View is the deterministic, read-only preview of a synchronization request.
type View struct {
	ProjectID       string              `json:"project_id"`
	Targets         []string            `json:"targets"`
	SelectionSHA256 string              `json:"selection_sha256"`
	ID              string              `json:"id"`
	Components      []synccatalog.Entry `json:"components"`
	Changes         []Change            `json:"changes"`
	Diagnostics     []Diagnostic        `json:"diagnostics,omitempty"`
}

// Applicable reports whether the preview has zero blocking diagnostics and an ID.
func (v View) Applicable() bool {
	return len(v.Diagnostics) == 0 && v.ID != ""
}

// safeRelPath reports whether p is a clean, local, forward-slash relative path
// without "..", backslashes, or Windows volume/drive prefixes.
func safeRelPath(p string) bool {
	if p == "" || p == "." || strings.HasPrefix(p, "/") || strings.Contains(p, "\\") || strings.Contains(p, ":") {
		return false
	}
	if path.Clean(p) != p || !filepath.IsLocal(filepath.FromSlash(p)) {
		return false
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return false
		}
	}
	return true
}

func sortedUniqueTargets(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, t := range in {
		t = strings.TrimSpace(t)
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}

func sortedComponentRefs(in []synccatalog.ComponentRef) []synccatalog.ComponentRef {
	out := append([]synccatalog.ComponentRef(nil), in...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		if out[i].ID != out[j].ID {
			return out[i].ID < out[j].ID
		}
		return out[i].Version < out[j].Version
	})
	return out
}

func sha256Hex(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// managedRulesBlock builds the deterministic Gandalf-managed block for sorted rule entries.
func managedRulesBlock(rules []synccatalog.Entry) []byte {
	var b strings.Builder
	b.WriteString(BlockStart + "\n")
	for _, r := range rules {
		fmt.Fprintf(&b, "## %s@%s\n", r.ID, r.Version)
		res := append([]synccatalog.Resource(nil), r.Resources...)
		sort.Slice(res, func(i, j int) bool { return res[i].Path < res[j].Path })
		for _, rc := range res {
			b.Write(rc.Bytes)
			if len(rc.Bytes) > 0 && rc.Bytes[len(rc.Bytes)-1] != '\n' {
				b.WriteByte('\n')
			}
		}
	}
	b.WriteString(BlockEnd + "\n")
	return []byte(b.String())
}

type plannedFile struct {
	path    string
	isRule  bool
	targets map[string]bool
	bytes   []byte
}

// Preview calculates a deterministic, read-only synchronization plan without modifying project files.
func Preview(
	project *syncfs.ScopedProject,
	catalog *synccatalog.Catalog,
	selection synccatalog.Selection,
	profiles []NativeProfile,
	receipts []Receipt,
) (View, error) {
	if project == nil {
		return View{}, errors.New("projectsync: nil project")
	}
	if catalog == nil {
		return View{}, errors.New("projectsync: nil catalog")
	}

	view := View{ProjectID: project.ID()}
	var diags []Diagnostic

	targets := sortedUniqueTargets(selection.Targets)
	view.Targets = targets
	if len(targets) == 0 {
		diags = append(diags, Diagnostic{Code: "missing_targets", Message: "no targets selected"})
	}

	profileByTarget := make(map[string]NativeProfile, len(profiles))
	for _, p := range profiles {
		profileByTarget[p.Target] = p
	}

	var verified []NativeProfile
	for _, t := range targets {
		p, ok := profileByTarget[t]
		if !ok || !p.Verified || !safeRelPath(p.SkillLocation) || !safeRelPath(p.RuleLocation) {
			diags = append(diags, Diagnostic{
				Code:    "unverified_target",
				Target:  t,
				Message: "target has no verified native profile with safe local locations",
			})
			continue
		}
		verified = append(verified, p)
	}

	entries, err := catalog.Resolve(selection)
	if err != nil {
		var derr *synccatalog.DiagnosticError
		if errors.As(err, &derr) {
			for _, d := range derr.Diagnostics {
				diags = append(diags, Diagnostic{
					Code:    d.Code,
					Message: d.Message,
				})
			}
		} else {
			return View{}, err
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Kind != entries[j].Kind {
			return entries[i].Kind < entries[j].Kind
		}
		if entries[i].ID != entries[j].ID {
			return entries[i].ID < entries[j].ID
		}
		return entries[i].Version < entries[j].Version
	})
	view.Components = entries

	refs := sortedComponentRefs(selection.Components)
	selParts := make([]string, 0, len(targets)+len(refs)*3)
	selParts = append(selParts, targets...)
	for _, r := range refs {
		selParts = append(selParts, string(r.Kind), r.ID, r.Version)
	}
	view.SelectionSHA256 = sha256Hex(selParts...)

	receiptByPath := make(map[string]Receipt, len(receipts))
	for _, r := range receipts {
		if r.Owner == OwnerGandalf {
			receiptByPath[r.Path] = r
		}
	}

	// Project verified targets' files. Patterns are reference-only.
	planned := map[string]*plannedFile{}
	addPlanned := func(rel string, isRule bool, target string, bytes []byte) {
		pf, ok := planned[rel]
		if !ok {
			pf = &plannedFile{path: rel, isRule: isRule, targets: map[string]bool{}, bytes: bytes}
			planned[rel] = pf
		} else if string(pf.bytes) != string(bytes) {
			diags = append(diags, Diagnostic{
				Code:    "path_collision",
				Path:    rel,
				Message: "multiple verified targets project different bytes to the same path",
			})
			return
		}
		pf.targets[target] = true
	}

	var ruleEntries []synccatalog.Entry
	for _, e := range entries {
		if e.Kind == synccatalog.KindRule {
			ruleEntries = append(ruleEntries, e)
		}
	}

	for _, p := range verified {
		for _, e := range entries {
			if e.Kind != synccatalog.KindSkill {
				continue
			}
			for _, res := range e.Resources {
				rel := path.Clean(p.SkillLocation + "/" + e.ID + "/" + res.Path)
				if !safeRelPath(rel) {
					diags = append(diags, Diagnostic{
						Code:    "unsafe_path",
						Target:  p.Target,
						Path:    rel,
						Message: "projected skill path is not a safe local relative path",
					})
					continue
				}
				addPlanned(rel, false, p.Target, res.Bytes)
			}
		}
		if len(ruleEntries) > 0 {
			rel := path.Clean(p.RuleLocation)
			if safeRelPath(rel) {
				addPlanned(rel, true, p.Target, managedRulesBlock(ruleEntries))
			}
		}
	}

	// Shared write deduplication / collision detection.
	paths := make([]string, 0, len(planned))
	for p := range planned {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	var changes []Change
	for _, p := range paths {
		pf := planned[p]
		before, err := project.Observe(p)
		if err != nil {
			if errors.Is(err, syncfs.ErrUnsafePath) {
				diags = append(diags, Diagnostic{
					Code:    "unsafe_path",
					Path:    p,
					Message: "projected path is unsafe or escapes project root",
				})
				continue
			}
			return View{}, err
		}
		receipt, hasReceipt := receiptByPath[p]

		var after []byte
		action := ActionCreate
		if pf.isRule {
			after, action, err = planRulesFile(p, before, pf.bytes, hasReceipt, receipt, &diags)
			if err != nil {
				return View{}, err
			}
		} else {
			after, action, err = planSkillFile(p, before, pf.bytes, hasReceipt, receipt, &diags)
			if err != nil {
				return View{}, err
			}
		}
		if action == "" {
			continue // no-op
		}
		afterState := syncfs.NewFileState(after)
		tg := make([]string, 0, len(pf.targets))
		for t := range pf.targets {
			tg = append(tg, t)
		}
		sort.Strings(tg)
		changes = append(changes, Change{
			Path:    p,
			Action:  action,
			Targets: tg,
			Before:  before,
			After:   afterState,
			Owner:   OwnerGandalf,
		})
	}

	// Precedence collisions (read-only).
	targetSet := map[string]bool{}
	for _, t := range targets {
		targetSet[t] = true
	}
	if targetSet["codex"] {
		if _, ok := planned["AGENTS.md"]; ok {
			st, err := project.Observe("AGENTS.override.md")
			if err != nil {
				if errors.Is(err, syncfs.ErrUnsafePath) {
					diags = append(diags, Diagnostic{
						Code:    "unsafe_path",
						Target:  "codex",
						Path:    "AGENTS.override.md",
						Message: "AGENTS.override.md is an unsafe path or symlink",
					})
				} else {
					return View{}, err
				}
			} else if st.Exists {
				diags = append(diags, Diagnostic{
					Code:    "precedence_conflict",
					Target:  "codex",
					Path:    "AGENTS.override.md",
					Message: "AGENTS.override.md exists and takes precedence over projected AGENTS.md",
				})
			}
		}
	}
	if targetSet["vibe"] {
		for _, e := range entries {
			if e.Kind != synccatalog.KindSkill {
				continue
			}
			p := path.Clean(".vibe/skills/" + e.ID + "/SKILL.md")
			st, err := project.Observe(p)
			if err != nil {
				if errors.Is(err, syncfs.ErrUnsafePath) {
					diags = append(diags, Diagnostic{
						Code:    "unsafe_path",
						Target:  "vibe",
						Path:    p,
						Message: "existing .vibe skill path is unsafe or a symlink",
					})
				} else {
					return View{}, err
				}
			} else if st.Exists {
				diags = append(diags, Diagnostic{
					Code:    "precedence_conflict",
					Target:  "vibe",
					Path:    p,
					Message: "existing .vibe skill takes precedence over projected .agents skill",
				})
			}
		}
	}

	view.Diagnostics = diags
	if len(diags) > 0 {
		view.ID = ""
		view.Changes = nil
		return view, nil
	}

	view.Changes = changes

	// Deterministic View.ID over canonical representation.
	var idParts []string
	idParts = append(idParts, project.ID(), view.SelectionSHA256)
	for _, e := range entries {
		idParts = append(idParts, string(e.Kind), e.ID, e.Version, e.Origin, e.SHA256)
	}
	profSorted := append([]NativeProfile(nil), verified...)
	sort.Slice(profSorted, func(i, j int) bool { return profSorted[i].Target < profSorted[j].Target })
	for _, p := range profSorted {
		idParts = append(idParts, p.Target, p.Version, p.OS, p.SkillLocation, p.RuleLocation)
	}
	for _, c := range changes {
		idParts = append(idParts,
			c.Path, c.Action, strings.Join(c.Targets, ","),
			fmt.Sprintf("%t", c.Before.Exists), c.Before.SHA256,
			fmt.Sprintf("%t", c.After.Exists), c.After.SHA256,
		)
	}
	view.ID = sha256Hex(idParts...)
	return view, nil
}

// planRulesFile computes the after-bytes and action for a shared rules file (e.g. AGENTS.md).
func planRulesFile(p string, before syncfs.FileState, block []byte, hasReceipt bool, receipt Receipt, diags *[]Diagnostic) ([]byte, string, error) {
	if !before.Exists {
		after := append([]byte(nil), block...)
		if len(after) == 0 || after[len(after)-1] != '\n' {
			after = append(after, '\n')
		}
		return after, ActionCreate, nil
	}
	data := before.Bytes
	startCount := strings.Count(string(data), BlockStart)
	endCount := strings.Count(string(data), BlockEnd)
	if startCount == 0 && endCount == 0 {
		sep := ""
		if len(data) > 0 && data[len(data)-1] != '\n' {
			sep = "\n"
		}
		after := append(append(append([]byte(nil), data...), []byte(sep)...), block...)
		return after, ActionUpdate, nil
	}
	startIdx := strings.Index(string(data), BlockStart)
	endIdx := strings.Index(string(data), BlockEnd)
	if startCount != 1 || endCount != 1 || endIdx < startIdx {
		*diags = append(*diags, Diagnostic{
			Code:    "marker_conflict",
			Path:    p,
			Message: "ambiguous Gandalf markers in shared rules file",
		})
		return nil, "", nil
	}
	if !hasReceipt || receipt.AfterSHA256 != before.SHA256 {
		*diags = append(*diags, Diagnostic{
			Code:    "ownership_conflict",
			Path:    p,
			Message: "existing managed block lacks a matching Gandalf receipt",
		})
		return nil, "", nil
	}
	endSpan := endIdx + len(BlockEnd)
	trimmedBlock := block
	if len(trimmedBlock) > 0 && trimmedBlock[len(trimmedBlock)-1] == '\n' {
		trimmedBlock = trimmedBlock[:len(trimmedBlock)-1]
	}
	after := append(append(append([]byte(nil), data[:startIdx]...), trimmedBlock...), data[endSpan:]...)
	if before.SHA256 == syncfs.NewFileState(after).SHA256 {
		return nil, "", nil // no-op
	}
	return after, ActionUpdate, nil
}

// planSkillFile computes the after-bytes and action for a dedicated skill file.
func planSkillFile(p string, before syncfs.FileState, desired []byte, hasReceipt bool, receipt Receipt, diags *[]Diagnostic) ([]byte, string, error) {
	if !before.Exists {
		return desired, ActionCreate, nil
	}
	if !hasReceipt || receipt.AfterSHA256 != before.SHA256 {
		*diags = append(*diags, Diagnostic{
			Code:    "ownership_conflict",
			Path:    p,
			Message: "existing skill file lacks a matching Gandalf receipt",
		})
		return nil, "", nil
	}
	if before.SHA256 == syncfs.NewFileState(desired).SHA256 {
		return nil, "", nil // no-op
	}
	return desired, ActionUpdate, nil
}
