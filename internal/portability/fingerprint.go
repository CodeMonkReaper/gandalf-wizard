package portability

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// Fingerprint pins every part of the shared case: the canonical definition
// bytes, the catalog, the resolved input, the memory copy with its revision and
// the two destinations. Two consumers of the case compute identical values, and
// the case fingerprint combines those parts.
type Fingerprint struct {
	Definition   string `json:"definition"`
	Catalog      string `json:"catalog"`
	Input        string `json:"input"`
	Memory       string `json:"memory"`
	Destinations string `json:"destinations"`
	Case         string `json:"case"`
}

// TextFingerprint returns the SHA-256 digest of a text.
func TextFingerprint(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

// DefinitionJSON returns the canonical bytes of the case definition. Every
// consumer writes exactly these bytes, so the engine persists one identity for
// the definition of the case.
func DefinitionJSON() ([]byte, error) {
	data, err := json.MarshalIndent(Definition(ReferenceConcurrency), "", "  ")
	if err != nil {
		return nil, fmt.Errorf("definición no serializable: %w", err)
	}
	return data, nil
}

// DefinitionFingerprint returns the digest of the definition bytes. It is the
// same SHA-256 the engine persists for the file it loads, so a consumer can
// compare the case with the definition of a real run. The bytes are not
// normalized: a file the engine rejects never matches the case.
func DefinitionFingerprint(data []byte) string {
	return TextFingerprint(string(data))
}

// CaseFingerprints computes the fingerprints of the whole case.
func CaseFingerprints() (Fingerprint, error) {
	def, err := DefinitionJSON()
	if err != nil {
		return Fingerprint{}, err
	}
	catalog, err := json.Marshal(Catalog())
	if err != nil {
		return Fingerprint{}, fmt.Errorf("catálogo no serializable: %w", err)
	}
	destinations, err := json.Marshal(Destinations())
	if err != nil {
		return Fingerprint{}, fmt.Errorf("destinos no serializables: %w", err)
	}
	fp := Fingerprint{
		Definition:   DefinitionFingerprint(def),
		Catalog:      TextFingerprint(string(catalog)),
		Input:        TextFingerprint(Input),
		Memory:       FixedMemory().Fingerprint,
		Destinations: TextFingerprint(string(destinations)),
	}
	fp.Case = TextFingerprint(strings.Join([]string{
		Revision, fp.Definition, fp.Catalog, fp.Input, fp.Memory, fp.Destinations,
	}, "|"))
	return fp, nil
}
