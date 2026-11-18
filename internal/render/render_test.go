package render

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/CodeMonkReaper/gandalf-wizard/internal/doctor"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/romero"
)

func renderToString(t *testing.T, fn func(buf *bytes.Buffer) error) string {
	t.Helper()
	var buf bytes.Buffer
	if err := fn(&buf); err != nil {
		t.Fatalf("render: error inesperado: %v", err)
	}
	return buf.String()
}

func TestRenderVersion(t *testing.T) {
	human := renderToString(t, func(buf *bytes.Buffer) error {
		return RenderVersion(buf, VersionView{Version: "0.1.0"}, false)
	})
	if human != "gandalf 0.1.0\n" {
		t.Errorf("human = %q, want %q", human, "gandalf 0.1.0\n")
	}

	out := renderToString(t, func(buf *bytes.Buffer) error {
		return RenderVersion(buf, VersionView{Version: "0.1.0"}, true)
	})
	want := "{\n  \"schema_version\": \"1\",\n  \"command\": \"version\",\n  \"version\": \"0.1.0\"\n}\n"
	if out != want {
		t.Errorf("json = %q, want %q", out, want)
	}
	var parsed VersionJSON
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("json inválido: %v", err)
	}
	if parsed.SchemaVersion != SchemaVersion {
		t.Errorf("schema_version = %q, want %q", parsed.SchemaVersion, SchemaVersion)
	}
}

func TestRenderConfigShow(t *testing.T) {
	view := ConfigShowView{
		Entries: []ConfigEntry{
			{Key: "project", Value: "faro", Origin: "proyecto"},
			{Key: "caller", Value: "luis", Origin: "global"},
			{Key: "romero.base_url", Value: "http://127.0.0.1:8420", Origin: "predeterminado"},
			{Key: "romero.timeout", Value: "10s", Origin: "predeterminado"},
			{Key: "credencial (ROMERO_API_KEY)", Value: "presente", Origin: "entorno"},
		},
		Warnings: []string{"clave desconocida \"extra\" en /tmp/.gandalf.json"},
	}

	human := renderToString(t, func(buf *bytes.Buffer) error {
		return RenderConfigShow(buf, view, false)
	})
	wantHuman := "| Clave | Valor | Origen |\n" +
		"| --- | --- | --- |\n" +
		"| project | faro | proyecto |\n" +
		"| caller | luis | global |\n" +
		"| romero.base_url | http://127.0.0.1:8420 | predeterminado |\n" +
		"| romero.timeout | 10s | predeterminado |\n" +
		"| credencial (ROMERO_API_KEY) | presente | entorno |\n" +
		"Aviso: clave desconocida \"extra\" en /tmp/.gandalf.json\n"
	if human != wantHuman {
		t.Errorf("human = %q, want %q", human, wantHuman)
	}

	out := renderToString(t, func(buf *bytes.Buffer) error {
		return RenderConfigShow(buf, view, true)
	})
	var parsed ConfigShowJSON
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("json inválido: %v", err)
	}
	if parsed.SchemaVersion != "1" || parsed.Command != "config show" {
		t.Errorf("schema/command = %q/%q", parsed.SchemaVersion, parsed.Command)
	}
	if len(parsed.Entries) != 5 || parsed.Entries[0].Key != "project" {
		t.Errorf("entries = %+v", parsed.Entries)
	}
	if len(parsed.Warnings) != 1 {
		t.Errorf("warnings = %+v", parsed.Warnings)
	}
}

func TestRenderDoctor(t *testing.T) {
	report := doctor.Report{
		Checks: []doctor.CheckResult{
			{Name: doctor.CheckConfig, Status: doctor.StatusOK, Detail: "proyecto: C:\\x\\.gandalf.json; global: no encontrado"},
			{Name: doctor.CheckProject, Status: doctor.StatusOK, Detail: "faro"},
			{Name: doctor.CheckRomeroURL, Status: doctor.StatusOK, Detail: "http://127.0.0.1:8420"},
			{Name: doctor.CheckCredential, Status: doctor.StatusFail, Detail: "ROMERO_API_KEY no está definida", Remediation: "Define ROMERO_API_KEY en el entorno de la sesión."},
			{Name: doctor.CheckRomeroConn, Status: doctor.StatusFail, Detail: "credencial rechazada o ausente (401)", Remediation: "Revisa ROMERO_API_KEY."},
			{Name: doctor.CheckHarness, Status: doctor.StatusOK, Detail: "codex: detectado · vibe: detectado · antigravity: no detectado · rsc: presente"},
		},
		Failures: 2,
		Warnings: 0,
	}

	human := renderToString(t, func(buf *bytes.Buffer) error {
		return RenderDoctor(buf, report, false)
	})
	wantHuman := "[OK]   configuración     proyecto: C:\\x\\.gandalf.json; global: no encontrado\n" +
		"[OK]   proyecto          faro\n" +
		"[OK]   romero.url        http://127.0.0.1:8420\n" +
		"[FAIL] credencial        ROMERO_API_KEY no está definida\n" +
		"       → Define ROMERO_API_KEY en el entorno de la sesión.\n" +
		"[FAIL] romero.conexión   credencial rechazada o ausente (401)\n" +
		"       → Revisa ROMERO_API_KEY.\n" +
		"[OK]   harness           codex: detectado · vibe: detectado · antigravity: no detectado · rsc: presente\n" +
		"Resultado: fallos=2 avisos=0\n"
	if human != wantHuman {
		t.Errorf("human = %q, want %q", human, wantHuman)
	}

	out := renderToString(t, func(buf *bytes.Buffer) error {
		return RenderDoctor(buf, report, true)
	})
	var parsed DoctorJSON
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("json inválido: %v", err)
	}
	if parsed.SchemaVersion != "1" || parsed.Command != "doctor" {
		t.Errorf("schema/command = %q/%q", parsed.SchemaVersion, parsed.Command)
	}
	if len(parsed.Checks) != 6 || parsed.Failures != 2 || parsed.Warnings != 0 {
		t.Errorf("checks/failures/warnings = %d/%d/%d", len(parsed.Checks), parsed.Failures, parsed.Warnings)
	}
}

func faroOutcome() romero.MemoryQueryOutcome {
	return romero.MemoryQueryOutcome{
		Status: romero.StatusOK,
		Entries: []romero.AttributedMemoryEntry{
			{
				Summary:          "Conservamos los informes para comparar entregas.",
				EpistemicState:   "observed",
				MemoryID:         "M1",
				SourceRefDisplay: "F1",
				VigenciaStatus:   romero.VigenciaActive,
			},
		},
		AuditTrail: romero.AuditTrail{Operations: []string{"search", "getDetail"}},
	}
}

func TestRenderMemoryQuery_OK(t *testing.T) {
	outcome := faroOutcome()
	view := MemoryQueryView{Project: "faro", Outcome: outcome, View: romero.Present(outcome)}

	human := renderToString(t, func(buf *bytes.Buffer) error {
		return RenderMemoryQuery(buf, view, false)
	})
	wantHuman := "Proyecto: faro\n\n" +
		"| Resumen atribuido a ROMERO | Estado declarado | Vigencia | Referencia del recuerdo | Referencia de la fuente original |\n" +
		"| --- | --- | --- | --- | --- |\n" +
		"| Conservamos los informes para comparar entregas. | observed | vigente | M1 | F1 |\n"
	if human != wantHuman {
		t.Errorf("human = %q, want %q", human, wantHuman)
	}

	out := renderToString(t, func(buf *bytes.Buffer) error {
		return RenderMemoryQuery(buf, view, true)
	})
	wantJSON := "{\n" +
		"  \"schema_version\": \"1\",\n" +
		"  \"command\": \"memory query\",\n" +
		"  \"project\": \"faro\",\n" +
		"  \"status\": \"ok\",\n" +
		"  \"status_text\": \"\",\n" +
		"  \"degraded\": false,\n" +
		"  \"degradation_notice\": \"\",\n" +
		"  \"entries\": [\n" +
		"    {\n" +
		"      \"summary\": \"Conservamos los informes para comparar entregas.\",\n" +
		"      \"epistemic_state\": \"observed\",\n" +
		"      \"vigencia\": \"vigente\",\n" +
		"      \"memory_ref\": \"M1\",\n" +
		"      \"source_ref\": \"F1\"\n" +
		"    }\n" +
		"  ]\n" +
		"}\n"
	if out != wantJSON {
		t.Errorf("json = %q, want %q", out, wantJSON)
	}
}

func TestRenderMemoryQuery_EmptyAndMissingSource(t *testing.T) {
	t.Run("empty muestra etiqueta y entries []", func(t *testing.T) {
		outcome := romero.MemoryQueryOutcome{Status: romero.StatusEmpty}
		view := MemoryQueryView{Project: "faro", Outcome: outcome, View: romero.Present(outcome)}

		human := renderToString(t, func(buf *bytes.Buffer) error {
			return RenderMemoryQuery(buf, view, false)
		})
		wantHuman := "Proyecto: faro\n\nSin coincidencias visibles\n"
		if human != wantHuman {
			t.Errorf("human = %q, want %q", human, wantHuman)
		}

		out := renderToString(t, func(buf *bytes.Buffer) error {
			return RenderMemoryQuery(buf, view, true)
		})
		var parsed MemoryQueryJSON
		if err := json.Unmarshal([]byte(out), &parsed); err != nil {
			t.Fatalf("json inválido: %v", err)
		}
		if parsed.Status != "empty" || parsed.StatusText != romero.LabelNoMatches {
			t.Errorf("status/status_text = %q/%q", parsed.Status, parsed.StatusText)
		}
		if parsed.Entries == nil || len(parsed.Entries) != 0 {
			t.Errorf("entries = %#v, want [] no null", parsed.Entries)
		}
		if !strings.Contains(out, "\"entries\": []") {
			t.Errorf("el JSON debe serializar entries como []: %q", out)
		}
	})

	t.Run("fuente ausente: null en JSON y etiqueta en tabla", func(t *testing.T) {
		outcome := romero.MemoryQueryOutcome{
			Status: romero.StatusOK,
			Entries: []romero.AttributedMemoryEntry{
				{Summary: "Sin fuente.", EpistemicState: "observed", MemoryID: "M2", VigenciaStatus: romero.VigenciaActive},
			},
		}
		view := MemoryQueryView{Project: "faro", Outcome: outcome, View: romero.Present(outcome)}

		human := renderToString(t, func(buf *bytes.Buffer) error {
			return RenderMemoryQuery(buf, view, false)
		})
		if !strings.Contains(human, "| Sin fuente. | observed | vigente | M2 | Fuente de origen no disponible |") {
			t.Errorf("tabla debe usar la etiqueta de fuente ausente: %q", human)
		}

		out := renderToString(t, func(buf *bytes.Buffer) error {
			return RenderMemoryQuery(buf, view, true)
		})
		var parsed MemoryQueryJSON
		if err := json.Unmarshal([]byte(out), &parsed); err != nil {
			t.Fatalf("json inválido: %v", err)
		}
		if len(parsed.Entries) != 1 || parsed.Entries[0].SourceRef != nil {
			t.Errorf("source_ref = %#v, want null", parsed.Entries[0].SourceRef)
		}
		if !strings.Contains(out, "\"source_ref\": null") {
			t.Errorf("el JSON debe serializar source_ref null: %q", out)
		}
	})
}

func TestRenderMemoryQuery_DeniedAndDegraded(t *testing.T) {
	t.Run("denegación sin contenido ni identificadores", func(t *testing.T) {
		outcome := romero.MemoryQueryOutcome{Status: romero.StatusAccessDenied}
		view := MemoryQueryView{Project: "faro", Outcome: outcome, View: romero.Present(outcome)}

		human := renderToString(t, func(buf *bytes.Buffer) error {
			return RenderMemoryQuery(buf, view, false)
		})
		if !strings.Contains(human, "Acceso denegado a la memoria solicitada") {
			t.Errorf("human debe mostrar la etiqueta de denegación: %q", human)
		}
		if strings.Contains(human, "M1") || strings.Contains(human, "Conservamos") {
			t.Errorf("la denegación no debe filtrar resúmenes ni IDs: %q", human)
		}

		out := renderToString(t, func(buf *bytes.Buffer) error {
			return RenderMemoryQuery(buf, view, true)
		})
		var parsed MemoryQueryJSON
		if err := json.Unmarshal([]byte(out), &parsed); err != nil {
			t.Fatalf("json inválido: %v", err)
		}
		if parsed.Status != "access_denied" || len(parsed.Entries) != 0 {
			t.Errorf("status/entries = %q/%d", parsed.Status, len(parsed.Entries))
		}
	})

	t.Run("degradado muestra el aviso", func(t *testing.T) {
		outcome := romero.MemoryQueryOutcome{
			Status:            romero.StatusPartial,
			Degraded:          true,
			DegradationNotice: "Búsqueda en modo degradado",
			Entries: []romero.AttributedMemoryEntry{
				{Summary: "Parcial.", EpistemicState: "observed", MemoryID: "M1", SourceRefDisplay: "F1", VigenciaStatus: romero.VigenciaActive},
			},
		}
		view := MemoryQueryView{Project: "faro", Outcome: outcome, View: romero.Present(outcome)}

		human := renderToString(t, func(buf *bytes.Buffer) error {
			return RenderMemoryQuery(buf, view, false)
		})
		if !strings.Contains(human, romero.LabelPartial) || !strings.Contains(human, "Búsqueda en modo degradado") {
			t.Errorf("human debe mostrar etiqueta parcial y aviso degradado: %q", human)
		}

		out := renderToString(t, func(buf *bytes.Buffer) error {
			return RenderMemoryQuery(buf, view, true)
		})
		var parsed MemoryQueryJSON
		if err := json.Unmarshal([]byte(out), &parsed); err != nil {
			t.Fatalf("json inválido: %v", err)
		}
		if !parsed.Degraded || parsed.DegradationNotice != "Búsqueda en modo degradado" {
			t.Errorf("degraded/notice = %v/%q", parsed.Degraded, parsed.DegradationNotice)
		}
	})

	t.Run("degradado sin aviso usa el respaldo del presentador en JSON", func(t *testing.T) {
		outcome := romero.MemoryQueryOutcome{
			Status:   romero.StatusEmpty,
			Degraded: true,
		}
		view := MemoryQueryView{Project: "faro", Outcome: outcome, View: romero.Present(outcome)}

		out := renderToString(t, func(buf *bytes.Buffer) error {
			return RenderMemoryQuery(buf, view, true)
		})
		var parsed MemoryQueryJSON
		if err := json.Unmarshal([]byte(out), &parsed); err != nil {
			t.Fatalf("json inválido: %v", err)
		}
		if !parsed.Degraded || parsed.DegradationNotice != "Búsqueda en modo degradado" {
			t.Errorf("degraded/notice = %v/%q, want true/Búsqueda en modo degradado", parsed.Degraded, parsed.DegradationNotice)
		}
	})
}
