package portability

import (
	"context"
	"errors"
	"fmt"

	"github.com/CodeMonkReaper/gandalf-wizard/internal/workflow/engine"
)

// MemoryQuery is the query agreed with the owner for the case.
const MemoryQuery = "decisiones previas propuesta"

// MemoryRevision pins the ROMERO revision the fixed copy comes from.
const MemoryRevision = "rev-3f46f9a"

// MemoryText is the fixed answer both destinations receive.
const MemoryText = "decision previa: continuar con la propuesta"

var (
	// ErrUnexpectedQuery reports a query other than the agreed one.
	ErrUnexpectedQuery = errors.New("portability: consulta no acordada")
	// ErrUnversionedCopy reports a copy without a fixed revision.
	ErrUnversionedCopy = errors.New("portability: copia sin revisión fijada")
	// ErrFingerprintMismatch reports a copy whose text, query or revision no
	// longer matches its own fingerprint.
	ErrFingerprintMismatch = errors.New("portability: huella de la copia alterada")
)

// MemoryCopy is the fixed, versioned copy of the ROMERO answer. Both
// destinations receive this copy, and each run records the revision.
type MemoryCopy struct {
	Query       string `json:"query"`
	Text        string `json:"text"`
	Revision    string `json:"revision"`
	Fingerprint string `json:"fingerprint"`
}

// FixedMemory returns the fixed copy of the case.
func FixedMemory() MemoryCopy {
	return MemoryCopy{
		Query:       MemoryQuery,
		Text:        MemoryText,
		Revision:    MemoryRevision,
		Fingerprint: MemoryFingerprint(MemoryQuery, MemoryText, MemoryRevision),
	}
}

// MemoryFingerprint returns the digest of the copy. The revision is part of it,
// so a copy that claims another ROMERO revision does not match the case.
func MemoryFingerprint(query, text, revision string) string {
	return TextFingerprint(fmt.Sprintf("%d:%s|%d:%s|%d:%s",
		len(query), query, len(text), text, len(revision), revision))
}

// fixedMemory answers only the agreed query with the fixed copy.
type fixedMemory struct{ copy MemoryCopy }

// NewFixedMemory validates the copy and returns a memory port that answers only
// the agreed query. A copy without a revision, or one whose content no longer
// matches its fingerprint, is refused.
func NewFixedMemory(copy MemoryCopy) (engine.MemoryPort, error) {
	if copy.Query != MemoryQuery {
		return nil, fmt.Errorf("%w: %q", ErrUnexpectedQuery, copy.Query)
	}
	if copy.Revision == "" {
		return nil, fmt.Errorf("%w: %q", ErrUnversionedCopy, copy.Query)
	}
	if copy.Fingerprint != MemoryFingerprint(copy.Query, copy.Text, copy.Revision) || copy.Fingerprint != FixedMemory().Fingerprint {
		return nil, ErrFingerprintMismatch
	}
	return &fixedMemory{copy: copy}, nil
}

// Query returns the fixed copy for the agreed query only.
func (m *fixedMemory) Query(ctx context.Context, _, _, query string) (engine.MemoryResult, error) {
	if err := ctx.Err(); err != nil {
		return engine.MemoryResult{}, err
	}
	if query != m.copy.Query {
		return engine.MemoryResult{}, fmt.Errorf("%w: %q", ErrUnexpectedQuery, query)
	}
	return engine.MemoryResult{Text: m.copy.Text}, nil
}
