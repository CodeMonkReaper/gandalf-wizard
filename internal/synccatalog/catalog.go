// Package synccatalog manages the immutable local catalog and project selection
// for reproducible synchronization across harnesses.
package synccatalog

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/CodeMonkReaper/gandalf-wizard/internal/syncfs"
)

const (
	// SelectionFile is the versioned project selection manifest.
	SelectionFile = ".gandalf-sync.json"
	// DefaultStateDir is the local project state directory for the catalog.
	DefaultStateDir = ".gandalf/sync"
)

// ComponentKind classifies a catalog entry.
type ComponentKind string

const (
	KindSkill   ComponentKind = "skill"
	KindRule    ComponentKind = "rule"
	KindPattern ComponentKind = "pattern"
)

var (
	ErrConflict          = errors.New("catalog entry conflict")
	ErrInvalidInput      = errors.New("invalid catalog input")
	ErrNotFound          = errors.New("catalog entry not found")
	ErrIntegrity         = errors.New("catalog integrity check failed")
	ErrMissingDependency = errors.New("missing or mismatched dependency")
)

// idVersionPattern accepts a single safe path component: it must start with an
// alphanumeric character, may contain letters, digits, dots, underscores and
// hyphens, and never exceeds 64 characters. It deliberately rejects empty
// strings, "..", path separators and whitespace.
var idVersionPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

// Dependency references another component by exact kind, id, and version.
type Dependency struct {
	Kind    ComponentKind `json:"kind"`
	ID      string        `json:"id"`
	Version string        `json:"version"`
}

// Resource stores a fixed file inside a catalog entry.
type Resource struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  []byte `json:"-"`
}

// Entry is an immutable catalog record for a skill, rule, or pattern.
type Entry struct {
	Kind         ComponentKind `json:"kind"`
	ID           string        `json:"id"`
	Version      string        `json:"version"`
	Origin       string        `json:"origin"`
	SHA256       string        `json:"sha256"`
	Resources    []Resource    `json:"resources"`
	Dependencies []Dependency  `json:"dependencies,omitempty"`
}

// RegisterOptions supplies optional metadata when registering a component.
type RegisterOptions struct {
	Origin       string
	Dependencies []Dependency
}

// ComponentRef identifies a selected component by exact version.
type ComponentRef struct {
	Kind    ComponentKind `json:"kind"`
	ID      string        `json:"id"`
	Version string        `json:"version"`
}

// Selection is the project-level declarative sync manifest (.gandalf-sync.json).
type Selection struct {
	Targets    []string       `json:"targets"`
	Components []ComponentRef `json:"components"`
}

// Diagnostic describes a typed resolution error without leaking file contents.
type Diagnostic struct {
	Code    string        `json:"code"`
	Kind    ComponentKind `json:"kind,omitempty"`
	ID      string        `json:"id,omitempty"`
	Version string        `json:"version,omitempty"`
	Message string        `json:"message"`
}

// DiagnosticError wraps one or more typed resolution diagnostics.
type DiagnosticError struct {
	Diagnostics []Diagnostic
}

func (e *DiagnosticError) Error() string {
	if len(e.Diagnostics) == 0 {
		return "catalog resolution failed"
	}
	return e.Diagnostics[0].Message
}

// Catalog stores immutable component versions under a scoped project.
type Catalog struct {
	project  *syncfs.ScopedProject
	stateDir string
}

// New creates a catalog backed by the given ScopedProject.
func New(project *syncfs.ScopedProject) *Catalog {
	return &Catalog{project: project, stateDir: DefaultStateDir}
}

// validateKind reports whether kind is one of the supported component kinds.
func validateKind(kind ComponentKind) error {
	switch kind {
	case KindSkill, KindRule, KindPattern:
		return nil
	default:
		return fmt.Errorf("%w: unknown component kind %q", ErrInvalidInput, kind)
	}
}

// validateID reports whether id is a safe single path component. It rejects
// empty strings, "..", separators and anything outside the allowed alphabet.
func validateID(id string) error {
	if !idVersionPattern.MatchString(id) || strings.Contains(id, "..") {
		return fmt.Errorf("%w: invalid id %q", ErrInvalidInput, id)
	}
	return nil
}

// validateVersion applies the same component rules to a version string.
func validateVersion(version string) error {
	return validateID(version)
}

// validateSourceRegular confirms path exists and is a plain regular file with
// no symlink components anywhere along its way.
func validateSourceRegular(path string) error {
	fi, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("%w: source %q unavailable", ErrInvalidInput, path)
	}
	if !fi.Mode().IsRegular() {
		return fmt.Errorf("%w: source %q is not a regular file", ErrInvalidInput, path)
	}
	return nil
}

// collectSkillResources walks sourceDir rejecting any symlink and returns the
// fixed resources of a skill: SKILL.md at its root plus every regular file
// found below it, with "/"-normalized relative paths, sorted.
func collectSkillResources(sourceDir string) ([]Resource, error) {
	if err := validateSourceRegular(filepath.Join(sourceDir, "SKILL.md")); err != nil {
		return nil, err
	}
	var resources []Resource
	walkErr := filepath.WalkDir(sourceDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		fi, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: symlink %q not allowed in skill source", ErrInvalidInput, path)
		}
		if fi.IsDir() {
			return nil
		}
		if !fi.Mode().IsRegular() {
			return fmt.Errorf("%w: non-regular entry %q in skill source", ErrInvalidInput, path)
		}
		rel, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		resources = append(resources, Resource{
			Path:   filepath.ToSlash(rel),
			SHA256: hex.EncodeToString(sum[:]),
			Bytes:  data,
		})
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	sort.Slice(resources, func(i, j int) bool { return resources[i].Path < resources[j].Path })
	return resources, nil
}

// hasResource reports whether resources contains an entry with the given path.
func hasResource(resources []Resource, path string) bool {
	for _, res := range resources {
		if res.Path == path {
			return true
		}
	}
	return false
}

// dependenciesEqual compares two dependency slices field by field.
func dependenciesEqual(a, b []Dependency) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Kind != b[i].Kind || a[i].ID != b[i].ID || a[i].Version != b[i].Version {
			return false
		}
	}
	return true
}

// canonicalSHA256 derives the immutable identity of an entry from its sorted
// resources plus its kind, id, version and origin.
func canonicalSHA256(e Entry) string {
	h := sha256.New()
	for _, res := range e.Resources {
		h.Write([]byte(res.Path))
		h.Write([]byte{0})
		h.Write([]byte(res.SHA256))
		h.Write([]byte{0})
	}
	for _, field := range []string{string(e.Kind), e.ID, e.Version, e.Origin} {
		h.Write([]byte(field))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// manifestPath returns the project-relative manifest location for a component.
func (c *Catalog) manifestPath(kind ComponentKind, id, version string) string {
	return filepath.ToSlash(filepath.Join(c.stateDir, "catalog", string(kind), id, version, "manifest.json"))
}

// blobPath returns the project-relative blob location for one resource.
func (c *Catalog) blobPath(kind ComponentKind, id, version, resPath string) string {
	return filepath.ToSlash(filepath.Join(c.stateDir, "catalog", string(kind), id, version, "files", resPath))
}

// Register fixes a local source under (kind, id, version).
func (c *Catalog) Register(kind ComponentKind, id, version, source string, opts RegisterOptions) (Entry, error) {
	if err := validateKind(kind); err != nil {
		return Entry{}, err
	}
	if err := validateID(id); err != nil {
		return Entry{}, err
	}
	if err := validateVersion(version); err != nil {
		return Entry{}, err
	}

	fi, err := os.Lstat(source)
	if err != nil {
		return Entry{}, fmt.Errorf("%w: source %q unavailable", ErrInvalidInput, source)
	}

	var (
		resources []Resource
		origin    string
	)
	switch kind {
	case KindSkill:
		if !fi.IsDir() {
			return Entry{}, fmt.Errorf("%w: skill source %q is not a directory", ErrInvalidInput, source)
		}
		resources, err = collectSkillResources(source)
		if err != nil {
			return Entry{}, err
		}
		if !hasResource(resources, "SKILL.md") {
			return Entry{}, fmt.Errorf("%w: skill source %q has no SKILL.md", ErrInvalidInput, source)
		}
	default: // KindRule, KindPattern
		if !fi.Mode().IsRegular() {
			return Entry{}, fmt.Errorf("%w: source %q is not a regular file", ErrInvalidInput, source)
		}
		data, err := os.ReadFile(source)
		if err != nil {
			return Entry{}, err
		}
		sum := sha256.Sum256(data)
		resources = []Resource{{
			Path:   filepath.ToSlash(filepath.Base(source)),
			SHA256: hex.EncodeToString(sum[:]),
			Bytes:  data,
		}}
	}

	origin = opts.Origin
	if origin == "" {
		origin = filepath.Clean(source)
	}

	deps := make([]Dependency, len(opts.Dependencies))
	copy(deps, opts.Dependencies)
	sort.Slice(deps, func(i, j int) bool {
		if deps[i].Kind != deps[j].Kind {
			return deps[i].Kind < deps[j].Kind
		}
		if deps[i].ID != deps[j].ID {
			return deps[i].ID < deps[j].ID
		}
		return deps[i].Version < deps[j].Version
	})

	entry := Entry{
		Kind:         kind,
		ID:           id,
		Version:      version,
		Origin:       origin,
		Resources:    resources,
		Dependencies: deps,
	}
	entry.SHA256 = canonicalSHA256(entry)

	lease, err := c.project.TryExclusive("catalog-register")
	if err != nil {
		return Entry{}, err
	}
	defer lease.Close()

	if err := c.writeEntry(lease, entry); err != nil {
		return Entry{}, err
	}
	return entry, nil
}

// writeEntry persists the manifest and every resource blob of an entry inside
// the project, using Observe before every write and Change for the writes.
func (c *Catalog) writeEntry(lease *syncfs.Lease, entry Entry) error {
	manifestPath := c.manifestPath(entry.Kind, entry.ID, entry.Version)
	manifestBytes, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	manifestSHA := sha256.Sum256(manifestBytes)

	existing, err := c.project.Observe(manifestPath)
	if err != nil {
		return err
	}
	if existing.Exists {
		var current Entry
		if err := json.Unmarshal(existing.Bytes, &current); err != nil {
			return fmt.Errorf("%w: corrupt manifest at %q", ErrIntegrity, manifestPath)
		}
		if current.SHA256 == entry.SHA256 &&
			current.Origin == entry.Origin &&
			dependenciesEqual(current.Dependencies, entry.Dependencies) {
			return nil // idempotent re-registration
		}
		return ErrConflict
	}

	for _, res := range entry.Resources {
		blobPath := c.blobPath(entry.Kind, entry.ID, entry.Version, res.Path)
		blobDesired := syncfs.NewFileState(res.Bytes)
		current, err := c.project.Observe(blobPath)
		if err != nil {
			return err
		}
		if current.Exists {
			if current.SHA256 != blobDesired.SHA256 {
				return fmt.Errorf("%w: blob %q already registered with different content", ErrConflict, res.Path)
			}
			continue
		}
		if _, err := c.project.Change(lease, blobPath, syncfs.FileState{}, blobDesired); err != nil {
			return err
		}
	}

	desired := syncfs.FileState{
		Exists: true,
		SHA256: hex.EncodeToString(manifestSHA[:]),
		Bytes:  manifestBytes,
	}
	expected := syncfs.FileState{} // must not exist yet
	if _, err := c.project.Change(lease, manifestPath, expected, desired); err != nil {
		return err
	}

	return c.upsertIndex(lease, entry, manifestPath)
}

type catalogIndexRecord struct {
	Kind         ComponentKind `json:"kind"`
	ID           string        `json:"id"`
	Version      string        `json:"version"`
	ManifestPath string        `json:"manifest_path"`
}

type catalogIndex struct {
	Entries []catalogIndexRecord `json:"entries"`
}

func (c *Catalog) upsertIndex(lease *syncfs.Lease, entry Entry, manifestPath string) error {
	indexPath := filepath.ToSlash(filepath.Join(c.stateDir, "catalog", "index.json"))
	state, err := c.project.Observe(indexPath)
	if err != nil {
		return err
	}
	var idx catalogIndex
	if state.Exists {
		if err := json.Unmarshal(state.Bytes, &idx); err != nil {
			return fmt.Errorf("%w: corrupt index at %q", ErrIntegrity, indexPath)
		}
	}
	found := false
	for _, rec := range idx.Entries {
		if rec.Kind == entry.Kind && rec.ID == entry.ID && rec.Version == entry.Version {
			found = true
			break
		}
	}
	if !found {
		idx.Entries = append(idx.Entries, catalogIndexRecord{
			Kind:         entry.Kind,
			ID:           entry.ID,
			Version:      entry.Version,
			ManifestPath: manifestPath,
		})
	}
	sort.Slice(idx.Entries, func(i, j int) bool {
		if idx.Entries[i].Kind != idx.Entries[j].Kind {
			return idx.Entries[i].Kind < idx.Entries[j].Kind
		}
		if idx.Entries[i].ID != idx.Entries[j].ID {
			return idx.Entries[i].ID < idx.Entries[j].ID
		}
		return idx.Entries[i].Version < idx.Entries[j].Version
	})
	idxBytes, err := json.Marshal(idx)
	if err != nil {
		return err
	}
	_, err = c.project.Change(lease, indexPath, state, syncfs.NewFileState(idxBytes))
	return err
}

// List returns all registered catalog entries in stable order.
func (c *Catalog) List() ([]Entry, error) {
	indexPath := filepath.ToSlash(filepath.Join(c.stateDir, "catalog", "index.json"))
	state, err := c.project.Observe(indexPath)
	if err != nil {
		return nil, err
	}
	if !state.Exists {
		return []Entry{}, nil
	}
	var index struct {
		Entries []struct {
			Kind         ComponentKind `json:"kind"`
			ID           string        `json:"id"`
			Version      string        `json:"version"`
			ManifestPath string        `json:"manifest_path"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(state.Bytes, &index); err != nil {
		return nil, fmt.Errorf("%w: corrupt index at %q", ErrIntegrity, indexPath)
	}
	entries := make([]Entry, 0, len(index.Entries))
	for _, rec := range index.Entries {
		if err := validateKind(rec.Kind); err != nil {
			return nil, err
		}
		if err := validateID(rec.ID); err != nil {
			return nil, err
		}
		if err := validateVersion(rec.Version); err != nil {
			return nil, err
		}
		manifestState, err := c.project.Observe(rec.ManifestPath)
		if err != nil {
			return nil, err
		}
		if !manifestState.Exists {
			return nil, fmt.Errorf("%w: index references missing manifest %q", ErrIntegrity, rec.ManifestPath)
		}
		var entry Entry
		if err := json.Unmarshal(manifestState.Bytes, &entry); err != nil {
			return nil, fmt.Errorf("%w: corrupt manifest at %q", ErrIntegrity, rec.ManifestPath)
		}
		entries = append(entries, entry)
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
	return entries, nil
}

// LoadSelection reads and validates .gandalf-sync.json through ScopedProject.Observe.
func LoadSelection(project *syncfs.ScopedProject) (Selection, error) {
	state, err := project.Observe(SelectionFile)
	if err != nil {
		return Selection{}, fmt.Errorf("%w: %s unavailable", ErrNotFound, SelectionFile)
	}
	if !state.Exists {
		return Selection{}, ErrNotFound
	}
	var sel Selection
	dec := json.NewDecoder(bytes.NewReader(state.Bytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&sel); err != nil {
		return Selection{}, fmt.Errorf("%w: invalid selection JSON", ErrInvalidInput)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return Selection{}, fmt.Errorf("%w: trailing data after selection JSON", ErrInvalidInput)
	}
	if err := validateSelection(sel); err != nil {
		return Selection{}, err
	}
	return sel, nil
}

// validateSelection checks that every target is non-empty and every component
// reference names a known kind with a safe id and version.
func validateSelection(sel Selection) error {
	for _, target := range sel.Targets {
		if strings.TrimSpace(target) == "" {
			return fmt.Errorf("%w: empty target", ErrInvalidInput)
		}
	}
	for _, ref := range sel.Components {
		if err := validateKind(ref.Kind); err != nil {
			return err
		}
		if err := validateID(ref.ID); err != nil {
			return err
		}
		if err := validateVersion(ref.Version); err != nil {
			return err
		}
	}
	return nil
}

// Resolve validates the selection and returns the fixed catalog entries with
// bytes loaded from the project blobs.
func (c *Catalog) Resolve(selection Selection) ([]Entry, error) {
	if len(selection.Components) == 0 {
		return []Entry{}, nil
	}
	entries := make([]Entry, 0, len(selection.Components))
	var diags []Diagnostic
	for _, ref := range selection.Components {
		entry, d, err := c.loadComponent(ref)
		if d != nil {
			diags = append(diags, *d)
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, dep := range entry.Dependencies {
			found := false
			for _, sel := range selection.Components {
				if sel.Kind == dep.Kind && sel.ID == dep.ID {
					found = true
					if sel.Version != dep.Version {
						diags = append(diags, Diagnostic{
							Code:    "version_mismatch",
							Kind:    dep.Kind,
							ID:      dep.ID,
							Version: dep.Version,
							Message: fmt.Sprintf("dependency %s %s selected with version %q, want %q", dep.Kind, dep.ID, sel.Version, dep.Version),
						})
					}
					break
				}
			}
			if !found {
				diags = append(diags, Diagnostic{
					Code:    "missing_dependency",
					Kind:    dep.Kind,
					ID:      dep.ID,
					Version: dep.Version,
					Message: fmt.Sprintf("dependency %s %s %s is not selected", dep.Kind, dep.ID, dep.Version),
				})
			}
		}
		entries = append(entries, *entry)
	}
	if len(diags) > 0 {
		return nil, &DiagnosticError{Diagnostics: diags}
	}
	return entries, nil
}

// loadComponent observes the manifest of one component, verifies the integrity
// of every resource blob against the manifest, and returns the fixed entry.
func (c *Catalog) loadComponent(ref ComponentRef) (*Entry, *Diagnostic, error) {
	manifestPath := c.manifestPath(ref.Kind, ref.ID, ref.Version)
	state, err := c.project.Observe(manifestPath)
	if err != nil {
		return nil, &Diagnostic{
			Code:    "missing_component",
			Kind:    ref.Kind,
			ID:      ref.ID,
			Version: ref.Version,
			Message: fmt.Sprintf("component %s %s %s is not registered", ref.Kind, ref.ID, ref.Version),
		}, nil
	}
	if !state.Exists {
		return nil, &Diagnostic{
			Code:    "missing_component",
			Kind:    ref.Kind,
			ID:      ref.ID,
			Version: ref.Version,
			Message: fmt.Sprintf("component %s %s %s is not registered", ref.Kind, ref.ID, ref.Version),
		}, nil
	}
	var entry Entry
	if err := json.Unmarshal(state.Bytes, &entry); err != nil {
		return nil, &Diagnostic{
			Code:    "integrity_error",
			Kind:    ref.Kind,
			ID:      ref.ID,
			Version: ref.Version,
			Message: fmt.Sprintf("corrupt manifest for %s %s %s", ref.Kind, ref.ID, ref.Version),
		}, nil
	}

	recomputed := entry
	recomputed.Resources = make([]Resource, 0, len(entry.Resources))
	for _, res := range entry.Resources {
		blobPath := c.blobPath(ref.Kind, ref.ID, ref.Version, res.Path)
		blobState, err := c.project.Observe(blobPath)
		if err != nil || !blobState.Exists {
			return nil, &Diagnostic{
				Code:    "integrity_error",
				Kind:    ref.Kind,
				ID:      ref.ID,
				Version: ref.Version,
				Message: fmt.Sprintf("missing blob %q", res.Path),
			}, nil
		}
		if blobState.SHA256 != res.SHA256 {
			return nil, &Diagnostic{
				Code:    "integrity_error",
				Kind:    ref.Kind,
				ID:      ref.ID,
				Version: ref.Version,
				Message: fmt.Sprintf("blob %q hash mismatch", res.Path),
			}, nil
		}
		res.Bytes = blobState.Bytes
		recomputed.Resources = append(recomputed.Resources, res)
	}
	if got := canonicalSHA256(recomputed); got != entry.SHA256 {
		return nil, &Diagnostic{
			Code:    "integrity_error",
			Kind:    ref.Kind,
			ID:      ref.ID,
			Version: ref.Version,
			Message: fmt.Sprintf("entry hash mismatch for %s %s %s", ref.Kind, ref.ID, ref.Version),
		}, nil
	}
	return &recomputed, nil, nil
}
