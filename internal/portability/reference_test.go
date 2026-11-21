package portability_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/CodeMonkReaper/gandalf-wizard/internal/portability"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/workflow/schema"
)

// Pinned fingerprints of revision 1 of the shared case. A change of the
// definition, the catalog, the input or the memory copy must bump
// portability.Revision and update these values together with the approved
// evidence. An unannounced change of the case fails this test: that is the
// control that stops a run from claiming the same revision without proof.
const (
	wantDefinition   = "598e197b52ffa23f9b73fe699f404d1af6f9deae639ede82af0266c5e895dc78"
	wantCatalog      = "89ffddd17ffc5476ba997ce69e608505413c655d1a4197ea84c8892b6be2bb68"
	wantInput        = "ff8b65d6786ded24db7dce80dbbb5657b7e9bb1d1b707c5ef567972d246a10b2"
	wantMemory       = "1664844657a974f07e1e5a90e3622059f19f4add4f896e17f5058023046bb32e"
	wantDestinations = "bd5909e9fb79f5ac7c92c8b9b3a532f71fa37bae4d047fb25ee433cc3cbc196c"
	wantCase         = "b5a37cc7fc7aaeee7b249e9201970a8358c17da75ef3d41401411cd5ee719149"
)

// TestPortableReference fixes the shared case of phase 04. The Runtime 0.3
// acceptance test and the portability test read the same versioned definition,
// catalog, input and memory copy, so their fingerprints are identical and match
// the pinned values. An unexpected query or a modified copy is refused.
func TestPortableReference(t *testing.T) {
	t.Run("huellas iguales y fijadas", func(t *testing.T) {
		first, err := portability.CaseFingerprints()
		if err != nil {
			t.Fatalf("huellas del caso: %v", err)
		}
		second, err := portability.CaseFingerprints()
		if err != nil {
			t.Fatalf("huellas del caso (segunda lectura): %v", err)
		}
		if first != second {
			t.Fatalf("el caso no es determinista:\n primero: %+v\n segundo: %+v", first, second)
		}
		for _, check := range []struct{ name, got, want string }{
			{"definición", first.Definition, wantDefinition},
			{"catálogo", first.Catalog, wantCatalog},
			{"entrada", first.Input, wantInput},
			{"memoria", first.Memory, wantMemory},
			{"destinos", first.Destinations, wantDestinations},
			{"caso", first.Case, wantCase},
		} {
			if check.got != check.want {
				t.Errorf("huella de %s = %s; want %s (¿cambió el caso sin subir la revisión?)",
					check.name, check.got, check.want)
			}
		}
	})

	t.Run("la definición escrita es la que el motor identifica", func(t *testing.T) {
		data, err := portability.DefinitionJSON()
		if err != nil {
			t.Fatalf("definición canónica: %v", err)
		}
		if got := portability.DefinitionFingerprint(data); got != wantDefinition {
			t.Errorf("huella de la definición canónica = %s; want %s", got, wantDefinition)
		}
		path := filepath.Join(t.TempDir(), "workflow.json")
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatalf("escribir la definición: %v", err)
		}
		validated, err := schema.LoadAndValidate(path, schema.ValidateOptions{})
		if err != nil {
			t.Fatalf("el motor rechaza la definición canónica: %v", err)
		}
		if validated.DefinitionSHA256 != wantDefinition {
			t.Errorf("identidad del motor = %s; want %s", validated.DefinitionSHA256, wantDefinition)
		}
		if validated.Definition.ID != portability.DefinitionID || validated.Definition.Version != portability.DefinitionVersion {
			t.Errorf("el motor identifica %s@%s; want %s@%s", validated.Definition.ID,
				validated.Definition.Version, portability.DefinitionID, portability.DefinitionVersion)
		}
	})

	t.Run("un cambio del caso cambia la huella", func(t *testing.T) {
		changed := portability.Definition(portability.ReferenceConcurrency)
		for i := range changed.Steps {
			if changed.Steps[i].Correction != nil {
				changed.Steps[i].Correction.MaxIterations++
			}
		}
		data, err := json.MarshalIndent(changed, "", "  ")
		if err != nil {
			t.Fatalf("definición alterada: %v", err)
		}
		if got := portability.DefinitionFingerprint(data); got == wantDefinition {
			t.Error("una definición con otro límite de corrección conserva la huella del caso")
		}
		if got := portability.MemoryFingerprint(portability.MemoryQuery, portability.MemoryText, "rev-otra"); got == wantMemory {
			t.Error("una copia con otra revisión conserva la huella del caso")
		}
		if got := portability.MemoryFingerprint(portability.MemoryQuery, portability.MemoryText+"+", portability.MemoryRevision); got == wantMemory {
			t.Error("una copia con otro texto conserva la huella del caso")
		}
	})

	t.Run("contenido versionado", func(t *testing.T) {
		def := portability.Definition(portability.ReferenceConcurrency)
		if def.ID != portability.DefinitionID || def.Version != portability.DefinitionVersion {
			t.Errorf("definición = %s@%s; want %s@%s",
				def.ID, def.Version, portability.DefinitionID, portability.DefinitionVersion)
		}
		if def.Objective.ID != portability.ObjectiveID || def.Objective.Version != portability.ObjectiveVersion {
			t.Errorf("objetivo = %s@%s; want %s@%s",
				def.Objective.ID, def.Objective.Version, portability.ObjectiveID, portability.ObjectiveVersion)
		}
		if def.ConcurrencyLimit != portability.ReferenceConcurrency {
			t.Errorf("límite de concurrencia = %d; want %d", def.ConcurrencyLimit, portability.ReferenceConcurrency)
		}
		got, want := portability.Destinations(), []portability.Destination{portability.Codex, portability.OpenCode}
		if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
			t.Errorf("destinos = %v; want %v", got, want)
		}
		copy := portability.FixedMemory()
		if copy.Query != portability.MemoryQuery {
			t.Errorf("consulta de la copia = %q; want %q", copy.Query, portability.MemoryQuery)
		}
		if copy.Revision != portability.MemoryRevision || copy.Text != portability.MemoryText {
			t.Errorf("la copia de memoria no está fijada: %+v", copy)
		}
		if copy.Fingerprint != portability.MemoryFingerprint(copy.Query, copy.Text, copy.Revision) {
			t.Error("la huella de la copia no corresponde a su contenido")
		}
		if got := len(portability.Catalog()); got != routerSteps(def) {
			t.Errorf("el catálogo fija %d pasos; la definición declara %d", got, routerSteps(def))
		}
		if portability.Input == "" || portability.Revision == "" {
			t.Error("la entrada o la revisión del caso están vacías")
		}
	})

	t.Run("el motor rechaza una definición con campos desconocidos", func(t *testing.T) {
		data, err := portability.DefinitionJSON()
		if err != nil {
			t.Fatalf("definición canónica: %v", err)
		}
		var fields map[string]any
		if err := json.Unmarshal(data, &fields); err != nil {
			t.Fatalf("definición canónica ilegible: %v", err)
		}
		fields["campo_desconocido"] = 1
		altered, err := json.Marshal(fields)
		if err != nil {
			t.Fatalf("definición alterada: %v", err)
		}
		path := filepath.Join(t.TempDir(), "workflow.json")
		if err := os.WriteFile(path, altered, 0o600); err != nil {
			t.Fatalf("escribir la definición alterada: %v", err)
		}
		if _, err := schema.LoadAndValidate(path, schema.ValidateOptions{}); err == nil {
			t.Fatal("el motor aceptó una definición con campos desconocidos")
		}
	})

	t.Run("la copia fija responde la consulta acordada", func(t *testing.T) {
		port, err := portability.NewFixedMemory(portability.FixedMemory())
		if err != nil {
			t.Fatalf("preparar la copia fija: %v", err)
		}
		result, err := port.Query(context.Background(), "demo", "test", portability.MemoryQuery)
		if err != nil {
			t.Fatalf("consulta acordada: %v", err)
		}
		if result.Text != portability.MemoryText {
			t.Errorf("texto devuelto = %q; want %q", result.Text, portability.MemoryText)
		}
	})

	t.Run("rechazo de una consulta distinta", func(t *testing.T) {
		port, err := portability.NewFixedMemory(portability.FixedMemory())
		if err != nil {
			t.Fatalf("preparar la copia fija: %v", err)
		}
		result, err := port.Query(context.Background(), "demo", "test", "otra consulta")
		if !errors.Is(err, portability.ErrUnexpectedQuery) {
			t.Fatalf("error = %v; want ErrUnexpectedQuery", err)
		}
		if result.Text != "" {
			t.Errorf("la consulta rechazada devolvió contenido: %q", result.Text)
		}
	})

	t.Run("rechazo de una huella distinta", func(t *testing.T) {
		altered := portability.FixedMemory()
		altered.Text += " (alterado)"
		if _, err := portability.NewFixedMemory(altered); !errors.Is(err, portability.ErrFingerprintMismatch) {
			t.Fatalf("error = %v; want ErrFingerprintMismatch", err)
		}
	})

	t.Run("rechazo de una copia sin revisión", func(t *testing.T) {
		unversioned := portability.FixedMemory()
		unversioned.Revision = ""
		unversioned.Fingerprint = portability.MemoryFingerprint(unversioned.Query, unversioned.Text, unversioned.Revision)
		if _, err := portability.NewFixedMemory(unversioned); !errors.Is(err, portability.ErrUnversionedCopy) {
			t.Fatalf("error = %v; want ErrUnversionedCopy", err)
		}
	})

	t.Run("rechazo de una copia distinta con huella recalculada", func(t *testing.T) {
		for _, change := range []func(*portability.MemoryCopy){
			func(c *portability.MemoryCopy) { c.Text += " alterado" },
			func(c *portability.MemoryCopy) { c.Revision = "otra-revision" },
		} {
			copy := portability.FixedMemory()
			change(&copy)
			copy.Fingerprint = portability.MemoryFingerprint(copy.Query, copy.Text, copy.Revision)
			if _, err := portability.NewFixedMemory(copy); !errors.Is(err, portability.ErrFingerprintMismatch) {
				t.Errorf("copia distinta aceptada: error = %v; want ErrFingerprintMismatch", err)
			}
		}
	})
}

// routerSteps counts the router tasks declared by the definition.
func routerSteps(def schema.WorkflowDefinition) int {
	count := 0
	for _, step := range def.Steps {
		if step.Kind == schema.StepKindRouterTask {
			count++
		}
	}
	return count
}
