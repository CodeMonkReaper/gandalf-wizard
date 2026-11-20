package schema

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeDefFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatalf("write definition %s: %v", name, err)
	}
	return p
}

func hasValidationCode(err error, code string) bool {
	var verrs *ValidationErrors
	if !errors.As(err, &verrs) {
		return false
	}
	for _, item := range verrs.Errors {
		if item.Code == code {
			return true
		}
	}
	return false
}

const validReferenceWorkflowJSON = `{
  "schema_version": "1",
  "id": "analisis-propuesta",
  "version": "1.0.0",
  "objective": {
    "id": "OBJ-PROP-001",
    "version": "1.0.0",
    "statement": "Analizar propuesta tecnica con contexto historico y riesgos",
    "constraints": ["solo texto", "sin secretos"],
    "deliverables": [
      {"name": "riesgos", "source": "borrador_riesgos.riesgos"},
      {"name": "recomendacion", "source": "sintesis.recomendacion"}
    ],
    "success_criteria": [
      {
        "id": "CRIT-RIESGOS",
        "deliverable": "riesgos",
        "rules": [{"kind": "non_empty"}]
      },
      {
        "id": "CRIT-REC",
        "deliverable": "recomendacion",
        "rules": [
          {"kind": "non_empty"},
          {"kind": "json_valid"},
          {"kind": "json_required_keys", "keys": ["decision", "riesgos"]}
        ]
      }
    ]
  },
  "steps": [
    {
      "id": "contexto_romero",
      "kind": "memory_query",
      "outputs": ["contexto"],
      "memory": {"query": "decisiones previas propuesta"}
    },
    {
      "id": "borrador_riesgos",
      "kind": "router_task",
      "outputs": ["riesgos"],
      "router": {"prompt": "Lista riesgos tecnicos", "complexity": "media", "risk": "bajo"}
    },
    {
      "id": "sintesis",
      "kind": "router_task",
      "depends_on": ["contexto_romero", "borrador_riesgos"],
      "inputs": {
        "contexto": "contexto_romero.contexto",
        "riesgos": "borrador_riesgos.riesgos"
      },
      "outputs": ["recomendacion"],
      "router": {"prompt": "Sintetiza en JSON con decision y riesgos"},
      "correction": {"gate_step_id": "gate_sintesis", "max_iterations": 2}
    },
    {
      "id": "gate_sintesis",
      "kind": "quality_gate",
      "depends_on": ["sintesis"],
      "gate": {
        "target": "sintesis.recomendacion",
        "rules": [
          {"kind": "non_empty"},
          {"kind": "json_valid"},
          {"kind": "json_required_keys", "keys": ["decision", "riesgos"]},
          {"kind": "no_secrets"}
        ]
      }
    },
    {
      "id": "aprobacion_revision",
      "kind": "human_approval",
      "depends_on": ["gate_sintesis"],
      "approval": {"prompt": "Revisar recomendacion y aprobar cierre"}
    }
  ]
}`

func TestWorkflowSchema(t *testing.T) {
	t.Run("referencia valida en solo lectura y orden topologico", func(t *testing.T) {
		dir := t.TempDir()
		path := writeDefFile(t, dir, "analisis.json", validReferenceWorkflowJSON)

		beforeEntries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("readdir before: %v", err)
		}

		got, err := LoadAndValidate(path, ValidateOptions{
			CheckProviders: true,
			MemoryReady:    true,
			RouterReady:    true,
		})
		if err != nil {
			t.Fatalf("LoadAndValidate failed: %v", err)
		}

		afterEntries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("readdir after: %v", err)
		}
		if len(beforeEntries) != len(afterEntries) {
			t.Fatalf("LoadAndValidate mutated directory: before=%d after=%d", len(beforeEntries), len(afterEntries))
		}

		sum := sha256.Sum256([]byte(validReferenceWorkflowJSON))
		wantSHA := hex.EncodeToString(sum[:])
		if got.DefinitionSHA256 != wantSHA {
			t.Fatalf("DefinitionSHA256=%q want %q", got.DefinitionSHA256, wantSHA)
		}
		if got.Definition.ConcurrencyLimit != DefaultConcurrencyLimit {
			t.Fatalf("ConcurrencyLimit=%d want %d", got.Definition.ConcurrencyLimit, DefaultConcurrencyLimit)
		}
		if !got.RequiresMemory || !got.RequiresRouter {
			t.Fatalf("RequiresMemory=%v RequiresRouter=%v want true,true", got.RequiresMemory, got.RequiresRouter)
		}
		if len(got.ExecutionOrder) != 5 {
			t.Fatalf("ExecutionOrder len=%d want 5: %v", len(got.ExecutionOrder), got.ExecutionOrder)
		}
		pos := map[string]int{}
		for i, id := range got.ExecutionOrder {
			pos[id] = i
		}
		if pos["contexto_romero"] > pos["sintesis"] || pos["borrador_riesgos"] > pos["sintesis"] || pos["sintesis"] > pos["gate_sintesis"] || pos["gate_sintesis"] > pos["aprobacion_revision"] {
			t.Fatalf("invalid topological order: %v", got.ExecutionOrder)
		}
	})

	t.Run("validacion condicional de proveedores S6", func(t *testing.T) {
		dir := t.TempDir()
		onlyRouter := writeDefFile(t, dir, "router-only.json", `{
  "schema_version": "1",
  "id": "solo-router",
  "version": "1.0.0",
  "objective": {
    "id": "OBJ-R",
    "version": "1.0.0",
    "statement": "Solo router",
    "deliverables": [{"name": "out", "source": "paso.out"}],
    "success_criteria": [{"id": "C1", "deliverable": "out", "rules": [{"kind": "non_empty"}]}]
  },
  "steps": [
    {"id": "paso", "kind": "router_task", "outputs": ["out"], "router": {"prompt": "Hola"}}
  ]
}`)
		got, err := LoadAndValidate(onlyRouter, ValidateOptions{
			CheckProviders: true,
			MemoryReady:    false,
			RouterReady:    true,
		})
		if err != nil {
			t.Fatalf("router-only with MemoryReady=false should pass, got: %v", err)
		}
		if got.RequiresMemory || !got.RequiresRouter {
			t.Fatalf("unexpected requirements: mem=%v router=%v", got.RequiresMemory, got.RequiresRouter)
		}
		if _, err := LoadAndValidate(onlyRouter, ValidateOptions{
			CheckProviders: true,
			MemoryReady:    true,
			RouterReady:    false,
		}); !hasValidationCode(err, "provider_unavailable") {
			t.Fatalf("expected provider_unavailable when RouterReady=false, got: %v", err)
		}

		onlyMemory := writeDefFile(t, dir, "memory-only.json", `{
  "schema_version": "1",
  "id": "solo-memoria",
  "version": "1.0.0",
  "objective": {
    "id": "OBJ-M",
    "version": "1.0.0",
    "statement": "Solo memoria",
    "deliverables": [{"name": "ctx", "source": "mem.ctx"}],
    "success_criteria": [{"id": "C1", "deliverable": "ctx", "rules": [{"kind": "non_empty"}]}]
  },
  "steps": [
    {"id": "mem", "kind": "memory_query", "outputs": ["ctx"], "memory": {"query": "decision"}}
  ]
}`)
		gotMem, err := LoadAndValidate(onlyMemory, ValidateOptions{
			CheckProviders: true,
			MemoryReady:    true,
			RouterReady:    false,
		})
		if err != nil {
			t.Fatalf("memory-only with RouterReady=false should pass, got: %v", err)
		}
		if !gotMem.RequiresMemory || gotMem.RequiresRouter {
			t.Fatalf("unexpected requirements: mem=%v router=%v", gotMem.RequiresMemory, gotMem.RequiresRouter)
		}
		if _, err := LoadAndValidate(onlyMemory, ValidateOptions{
			CheckProviders: true,
			MemoryReady:    false,
			RouterReady:    true,
		}); !hasValidationCode(err, "provider_unavailable") {
			t.Fatalf("expected provider_unavailable when MemoryReady=false, got: %v", err)
		}
	})

	t.Run("rechazos tipados de esquema y grafo", func(t *testing.T) {
		tests := []struct {
			name     string
			jsonBody string
			wantCode string
		}{
			{
				name: "campo desconocido",
				jsonBody: `{
  "schema_version": "1",
  "extra_field": true,
  "id": "bad",
  "version": "1.0.0",
  "objective": {"id": "O1", "version": "1.0.0", "statement": "S", "deliverables": [{"name": "d", "source": "s.o"}], "success_criteria": [{"id": "C1", "deliverable": "d", "rules": [{"kind": "non_empty"}]}]},
  "steps": [{"id": "s", "kind": "router_task", "outputs": ["o"], "router": {"prompt": "p"}}]
}`,
				wantCode: "invalid_json",
			},
			{
				name: "tokens extra tras objeto JSON",
				jsonBody: `{
  "schema_version": "1",
  "id": "bad",
  "version": "1.0.0",
  "objective": {"id": "O1", "version": "1.0.0", "statement": "S", "deliverables": [{"name": "d", "source": "s.o"}], "success_criteria": [{"id": "C1", "deliverable": "d", "rules": [{"kind": "non_empty"}]}]},
  "steps": [{"id": "s", "kind": "router_task", "outputs": ["o"], "router": {"prompt": "p"}}]
}{"trailing":1}`,
				wantCode: "invalid_json",
			},
			{
				name: "version de esquema no soportada",
				jsonBody: `{
  "schema_version": "2",
  "id": "bad",
  "version": "1.0.0",
  "objective": {"id": "O1", "version": "1.0.0", "statement": "S", "deliverables": [{"name": "d", "source": "s.o"}], "success_criteria": [{"id": "C1", "deliverable": "d", "rules": [{"kind": "non_empty"}]}]},
  "steps": [{"id": "s", "kind": "router_task", "outputs": ["o"], "router": {"prompt": "p"}}]
}`,
				wantCode: "invalid_schema_version",
			},
			{
				name: "objetivo incompleto sin criterios",
				jsonBody: `{
  "schema_version": "1",
  "id": "bad",
  "version": "1.0.0",
  "objective": {"id": "O1", "version": "1.0.0", "statement": "S", "deliverables": [{"name": "d", "source": "s.o"}], "success_criteria": []},
  "steps": [{"id": "s", "kind": "router_task", "outputs": ["o"], "router": {"prompt": "p"}}]
}`,
				wantCode: "invalid_objective",
			},
			{
				name: "ciclo en depends_on",
				jsonBody: `{
  "schema_version": "1",
  "id": "ciclo",
  "version": "1.0.0",
  "objective": {"id": "O1", "version": "1.0.0", "statement": "S", "deliverables": [{"name": "d", "source": "a.o"}], "success_criteria": [{"id": "C1", "deliverable": "d", "rules": [{"kind": "non_empty"}]}]},
  "steps": [
    {"id": "a", "kind": "router_task", "depends_on": ["b"], "outputs": ["o"], "router": {"prompt": "p"}},
    {"id": "b", "kind": "router_task", "depends_on": ["a"], "outputs": ["o"], "router": {"prompt": "p"}}
  ]
}`,
				wantCode: "cycle_detected",
			},
			{
				name: "dependencia inexistente",
				jsonBody: `{
  "schema_version": "1",
  "id": "dep-rota",
  "version": "1.0.0",
  "objective": {"id": "O1", "version": "1.0.0", "statement": "S", "deliverables": [{"name": "d", "source": "a.o"}], "success_criteria": [{"id": "C1", "deliverable": "d", "rules": [{"kind": "non_empty"}]}]},
  "steps": [
    {"id": "a", "kind": "router_task", "depends_on": ["no_existe"], "outputs": ["o"], "router": {"prompt": "p"}}
  ]
}`,
				wantCode: "broken_dependency",
			},
			{
				name: "input apunta a salida inexistente o no ancestro",
				jsonBody: `{
  "schema_version": "1",
  "id": "input-roto",
  "version": "1.0.0",
  "objective": {"id": "O1", "version": "1.0.0", "statement": "S", "deliverables": [{"name": "d", "source": "b.o"}], "success_criteria": [{"id": "C1", "deliverable": "d", "rules": [{"kind": "non_empty"}]}]},
  "steps": [
    {"id": "a", "kind": "router_task", "outputs": ["o"], "router": {"prompt": "p"}},
    {"id": "b", "kind": "router_task", "inputs": {"x": "a.o"}, "outputs": ["o"], "router": {"prompt": "p"}}
  ]
}`,
				wantCode: "broken_reference",
			},
			{
				name: "ciclo de correccion sin max_iterations positivo",
				jsonBody: `{
  "schema_version": "1",
  "id": "corr-bad-iter",
  "version": "1.0.0",
  "objective": {"id": "O1", "version": "1.0.0", "statement": "S", "deliverables": [{"name": "d", "source": "r.o"}], "success_criteria": [{"id": "C1", "deliverable": "d", "rules": [{"kind": "non_empty"}]}]},
  "steps": [
    {"id": "r", "kind": "router_task", "outputs": ["o"], "router": {"prompt": "p"}, "correction": {"gate_step_id": "g", "max_iterations": 0}},
    {"id": "g", "kind": "quality_gate", "depends_on": ["r"], "gate": {"target": "r.o", "rules": [{"kind": "non_empty"}]}}
  ]
}`,
				wantCode: "invalid_correction",
			},
			{
				name: "correccion apuntando a gate informativo",
				jsonBody: `{
  "schema_version": "1",
  "id": "corr-info-gate",
  "version": "1.0.0",
  "objective": {"id": "O1", "version": "1.0.0", "statement": "S", "deliverables": [{"name": "d", "source": "r.o"}], "success_criteria": [{"id": "C1", "deliverable": "d", "rules": [{"kind": "non_empty"}]}]},
  "steps": [
    {"id": "r", "kind": "router_task", "outputs": ["o"], "router": {"prompt": "p"}, "correction": {"gate_step_id": "g", "max_iterations": 2}},
    {"id": "g", "kind": "quality_gate", "depends_on": ["r"], "gate": {"target": "r.o", "informative": true, "rules": [{"kind": "non_empty"}]}}
  ]
}`,
				wantCode: "invalid_correction",
			},
			{
				name: "correccion apuntando a gate que no depende del router_task",
				jsonBody: `{
  "schema_version": "1",
  "id": "corr-no-dep",
  "version": "1.0.0",
  "objective": {"id": "O1", "version": "1.0.0", "statement": "S", "deliverables": [{"name": "d", "source": "r.o"}], "success_criteria": [{"id": "C1", "deliverable": "d", "rules": [{"kind": "non_empty"}]}]},
  "steps": [
    {"id": "other", "kind": "router_task", "outputs": ["o"], "router": {"prompt": "p"}},
    {"id": "r", "kind": "router_task", "outputs": ["o"], "router": {"prompt": "p"}, "correction": {"gate_step_id": "g", "max_iterations": 2}},
    {"id": "g", "kind": "quality_gate", "depends_on": ["other"], "gate": {"target": "other.o", "rules": [{"kind": "non_empty"}]}}
  ]
}`,
				wantCode: "invalid_correction",
			},
			{
				name: "no_secrets marcado como informativo",
				jsonBody: `{
  "schema_version": "1",
  "id": "nosec-info",
  "version": "1.0.0",
  "objective": {"id": "O1", "version": "1.0.0", "statement": "S", "deliverables": [{"name": "d", "source": "r.o"}], "success_criteria": [{"id": "C1", "deliverable": "d", "rules": [{"kind": "non_empty"}]}]},
  "steps": [
    {"id": "r", "kind": "router_task", "outputs": ["o"], "router": {"prompt": "p"}},
    {"id": "g", "kind": "quality_gate", "depends_on": ["r"], "gate": {"target": "r.o", "informative": true, "rules": [{"kind": "no_secrets"}]}}
  ]
}`,
				wantCode: "non_informative_required",
			},
			{
				name: "criterio de objetivo marcado como informativo",
				jsonBody: `{
  "schema_version": "1",
  "id": "obj-info",
  "version": "1.0.0",
  "objective": {"id": "O1", "version": "1.0.0", "statement": "S", "deliverables": [{"name": "d", "source": "r.o"}], "success_criteria": [{"id": "C1", "deliverable": "d", "informative": true, "rules": [{"kind": "non_empty"}]}]},
  "steps": [
    {"id": "r", "kind": "router_task", "outputs": ["o"], "router": {"prompt": "p"}}
  ]
}`,
				wantCode: "non_informative_required",
			},
			{
				name: "regex invalido en gate",
				jsonBody: `{
  "schema_version": "1",
  "id": "bad-regex",
  "version": "1.0.0",
  "objective": {"id": "O1", "version": "1.0.0", "statement": "S", "deliverables": [{"name": "d", "source": "r.o"}], "success_criteria": [{"id": "C1", "deliverable": "d", "rules": [{"kind": "non_empty"}]}]},
  "steps": [
    {"id": "r", "kind": "router_task", "outputs": ["o"], "router": {"prompt": "p"}},
    {"id": "g", "kind": "quality_gate", "depends_on": ["r"], "gate": {"target": "r.o", "rules": [{"kind": "regex_match", "value": "[unclosed"}]}}
  ]
}`,
				wantCode: "invalid_rule",
			},
			{
				name: "identificador de paso con traversal",
				jsonBody: `{
  "schema_version": "1",
  "id": "bad-id",
  "version": "1.0.0",
  "objective": {"id": "O1", "version": "1.0.0", "statement": "S", "deliverables": [{"name": "d", "source": "../escape.o"}], "success_criteria": [{"id": "C1", "deliverable": "d", "rules": [{"kind": "non_empty"}]}]},
  "steps": [
    {"id": "../escape", "kind": "router_task", "outputs": ["o"], "router": {"prompt": "p"}}
  ]
}`,
				wantCode: "invalid_identifier",
			},
			{
				name: "concurrency_limit fuera de rango",
				jsonBody: `{
  "schema_version": "1",
  "id": "bad-conc",
  "version": "1.0.0",
  "concurrency_limit": 99,
  "objective": {"id": "O1", "version": "1.0.0", "statement": "S", "deliverables": [{"name": "d", "source": "r.o"}], "success_criteria": [{"id": "C1", "deliverable": "d", "rules": [{"kind": "non_empty"}]}]},
  "steps": [
    {"id": "r", "kind": "router_task", "outputs": ["o"], "router": {"prompt": "p"}}
  ]
}`,
				wantCode: "invalid_concurrency",
			},
			{
				name: "paso duplicado",
				jsonBody: `{
  "schema_version": "1",
  "id": "dup-step",
  "version": "1.0.0",
  "objective": {"id": "O1", "version": "1.0.0", "statement": "S", "deliverables": [{"name": "d", "source": "r.o"}], "success_criteria": [{"id": "C1", "deliverable": "d", "rules": [{"kind": "non_empty"}]}]},
  "steps": [
    {"id": "r", "kind": "router_task", "outputs": ["o"], "router": {"prompt": "p"}},
    {"id": "r", "kind": "router_task", "outputs": ["o"], "router": {"prompt": "q"}}
  ]
}`,
				wantCode: "duplicate_step",
			},
			{
				name: "auto dependencia",
				jsonBody: `{
  "schema_version": "1",
  "id": "self-dep",
  "version": "1.0.0",
  "objective": {"id": "O1", "version": "1.0.0", "statement": "S", "deliverables": [{"name": "d", "source": "r.o"}], "success_criteria": [{"id": "C1", "deliverable": "d", "rules": [{"kind": "non_empty"}]}]},
  "steps": [
    {"id": "r", "kind": "router_task", "depends_on": ["r"], "outputs": ["o"], "router": {"prompt": "p"}}
  ]
}`,
				wantCode: "cycle_detected",
			},
			{
				name: "deliverable con salida inexistente",
				jsonBody: `{
  "schema_version": "1",
  "id": "bad-deliv",
  "version": "1.0.0",
  "objective": {"id": "O1", "version": "1.0.0", "statement": "S", "deliverables": [{"name": "d", "source": "r.missing"}], "success_criteria": [{"id": "C1", "deliverable": "d", "rules": [{"kind": "non_empty"}]}]},
  "steps": [
    {"id": "r", "kind": "router_task", "outputs": ["o"], "router": {"prompt": "p"}}
  ]
}`,
				wantCode: "broken_reference",
			},
			{
				name: "dos router_task comparten gate de correccion",
				jsonBody: `{
  "schema_version": "1",
  "id": "shared-corr",
  "version": "1.0.0",
  "objective": {"id": "O1", "version": "1.0.0", "statement": "S", "deliverables": [{"name": "d", "source": "r1.o"}], "success_criteria": [{"id": "C1", "deliverable": "d", "rules": [{"kind": "non_empty"}]}]},
  "steps": [
    {"id": "r1", "kind": "router_task", "outputs": ["o"], "router": {"prompt": "p1"}, "correction": {"gate_step_id": "g", "max_iterations": 2}},
    {"id": "r2", "kind": "router_task", "outputs": ["o"], "router": {"prompt": "p2"}, "correction": {"gate_step_id": "g", "max_iterations": 2}},
    {"id": "g", "kind": "quality_gate", "depends_on": ["r1", "r2"], "gate": {"target": "r1.o", "rules": [{"kind": "non_empty"}]}}
  ]
}`,
				wantCode: "invalid_correction",
			},
			{
				name: "correccion apuntando a gate que evalua otro paso paralelo",
				jsonBody: `{
  "schema_version": "1",
  "id": "corr-wrong-target-parallel",
  "version": "1.0.0",
  "objective": {"id": "O1", "version": "1.0.0", "statement": "S", "deliverables": [{"name": "d", "source": "r.o"}], "success_criteria": [{"id": "C1", "deliverable": "d", "rules": [{"kind": "non_empty"}]}]},
  "steps": [
    {"id": "other", "kind": "router_task", "outputs": ["o"], "router": {"prompt": "p1"}},
    {"id": "r", "kind": "router_task", "outputs": ["o"], "router": {"prompt": "p2"}, "correction": {"gate_step_id": "g", "max_iterations": 2}},
    {"id": "g", "kind": "quality_gate", "depends_on": ["r", "other"], "gate": {"target": "other.o", "rules": [{"kind": "non_empty"}]}}
  ]
}`,
				wantCode: "invalid_correction",
			},
			{
				name: "correccion apuntando a gate que evalua ancestro transitivo",
				jsonBody: `{
  "schema_version": "1",
  "id": "corr-wrong-target-ancestor",
  "version": "1.0.0",
  "objective": {"id": "O1", "version": "1.0.0", "statement": "S", "deliverables": [{"name": "d", "source": "r.o"}], "success_criteria": [{"id": "C1", "deliverable": "d", "rules": [{"kind": "non_empty"}]}]},
  "steps": [
    {"id": "other", "kind": "router_task", "outputs": ["o"], "router": {"prompt": "p1"}},
    {"id": "r", "kind": "router_task", "depends_on": ["other"], "outputs": ["o"], "router": {"prompt": "p2"}, "correction": {"gate_step_id": "g", "max_iterations": 2}},
    {"id": "g", "kind": "quality_gate", "depends_on": ["r"], "gate": {"target": "other.o", "rules": [{"kind": "non_empty"}]}}
  ]
}`,
				wantCode: "invalid_correction",
			},
			{
				name: "complexity y risk invalidos en router_task",
				jsonBody: `{
  "schema_version": "1",
  "id": "bad-router-meta",
  "version": "1.0.0",
  "objective": {"id": "O1", "version": "1.0.0", "statement": "S", "deliverables": [{"name": "d", "source": "r.o"}], "success_criteria": [{"id": "C1", "deliverable": "d", "rules": [{"kind": "non_empty"}]}]},
  "steps": [
    {"id": "r", "kind": "router_task", "outputs": ["o"], "router": {"prompt": "p", "complexity": "extrema", "risk": "critico"}}
  ]
}`,
				wantCode: "invalid_step",
			},
			{
				name: "bloque de configuracion ajeno al kind",
				jsonBody: `{
  "schema_version": "1",
  "id": "mixed-kind",
  "version": "1.0.0",
  "objective": {"id": "O1", "version": "1.0.0", "statement": "S", "deliverables": [{"name": "d", "source": "r.o"}], "success_criteria": [{"id": "C1", "deliverable": "d", "rules": [{"kind": "non_empty"}]}]},
  "steps": [
    {"id": "r", "kind": "router_task", "outputs": ["o"], "router": {"prompt": "p"}, "memory": {"query": "x"}}
  ]
}`,
				wantCode: "invalid_step",
			},
			{
				name: "min_length con limit cero",
				jsonBody: `{
  "schema_version": "1",
  "id": "bad-minlen",
  "version": "1.0.0",
  "objective": {"id": "O1", "version": "1.0.0", "statement": "S", "deliverables": [{"name": "d", "source": "r.o"}], "success_criteria": [{"id": "C1", "deliverable": "d", "rules": [{"kind": "min_length", "limit": 0}]}]},
  "steps": [
    {"id": "r", "kind": "router_task", "outputs": ["o"], "router": {"prompt": "p"}}
  ]
}`,
				wantCode: "invalid_rule",
			},
			{
				name: "json_required_keys vacio",
				jsonBody: `{
  "schema_version": "1",
  "id": "bad-jsonkeys",
  "version": "1.0.0",
  "objective": {"id": "O1", "version": "1.0.0", "statement": "S", "deliverables": [{"name": "d", "source": "r.o"}], "success_criteria": [{"id": "C1", "deliverable": "d", "rules": [{"kind": "json_required_keys", "keys": []}]}]},
  "steps": [
    {"id": "r", "kind": "router_task", "outputs": ["o"], "router": {"prompt": "p"}}
  ]
}`,
				wantCode: "invalid_rule",
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				dir := t.TempDir()
				p := writeDefFile(t, dir, "def.json", tc.jsonBody)
				_, err := LoadAndValidate(p, ValidateOptions{})
				if err == nil {
					t.Fatalf("expected error code %s, got nil", tc.wantCode)
				}
				if !hasValidationCode(err, tc.wantCode) {
					t.Fatalf("expected error code %s, got: %v", tc.wantCode, err)
				}
			})
		}
	})

	t.Run("workflow sin proveedores S6 pasa con ambos false", func(t *testing.T) {
		dir := t.TempDir()
		p := writeDefFile(t, dir, "approval-only.json", `{
  "schema_version": "1",
  "id": "solo-aprobacion",
  "version": "1.0.0",
  "objective": {
    "id": "OBJ-A",
    "version": "1.0.0",
    "statement": "Sin memory_query ni router_task",
    "deliverables": [{"name": "acta", "source": "aprob.acta"}],
    "success_criteria": [{"id": "C1", "deliverable": "acta", "rules": [{"kind": "non_empty"}]}]
  },
  "steps": [
    {"id": "aprob", "kind": "human_approval", "outputs": ["acta"], "approval": {"prompt": "Confirmar"}},
    {"id": "g", "kind": "quality_gate", "depends_on": ["aprob"], "gate": {"target": "aprob.acta", "rules": [{"kind": "non_empty"}]}}
  ]
}`)
		got, err := LoadAndValidate(p, ValidateOptions{
			CheckProviders: true,
			MemoryReady:    false,
			RouterReady:    false,
		})
		if err != nil {
			t.Fatalf("expected no-provider workflow to pass S6, got: %v", err)
		}
		if got.RequiresMemory || got.RequiresRouter {
			t.Fatalf("expected RequiresMemory=false RequiresRouter=false, got %v,%v", got.RequiresMemory, got.RequiresRouter)
		}
	})

	t.Run("archivo inexistente directorio o mayor a 1 MiB retorna io_error", func(t *testing.T) {
		dir := t.TempDir()
		_, err := LoadAndValidate(filepath.Join(dir, "missing.json"), ValidateOptions{})
		if !hasValidationCode(err, "io_error") {
			t.Fatalf("expected io_error for missing file, got: %v", err)
		}
		if strings.TrimSpace(err.Error()) == "" {
			t.Fatal("expected non-empty error string")
		}

		if _, err := LoadAndValidate(dir, ValidateOptions{}); !hasValidationCode(err, "io_error") {
			t.Fatalf("expected io_error for directory path, got: %v", err)
		}

		oversized := writeDefFile(t, dir, "big.json", strings.Repeat(" ", (1<<20)+8))
		if _, err := LoadAndValidate(oversized, ValidateOptions{}); !hasValidationCode(err, "io_error") {
			t.Fatalf("expected io_error for >1MiB file, got: %v", err)
		}
	})
}
