package romero

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestIntegration_RomeroRealInstance runs against a live ROMERO instance.
// Skipped unless ROMERO_BASE_URL, ROMERO_API_KEY and ROMERO_PROJECT_ID are set.
func TestIntegration_RomeroRealInstance(t *testing.T) {
	baseURL := os.Getenv("ROMERO_BASE_URL")
	apiKey := os.Getenv("ROMERO_API_KEY")
	projectID := os.Getenv("ROMERO_PROJECT_ID")
	if baseURL == "" || apiKey == "" || projectID == "" {
		t.Skip("ROMERO_BASE_URL/ROMERO_API_KEY/ROMERO_PROJECT_ID not set; skipping live integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	adapter := NewRomeroPublicClientAdapter(baseURL, nil, func() string { return apiKey })
	orch := NewMemoryQueryOrchestrator(adapter)

	ws := WorkspaceContext{ActiveProjectID: projectID, CallerIdentity: "integration-test"}
	outcome := orch.QueryProjectMemory(ctx, ws, "informes de prueba")

	if outcome.Status != StatusOK && outcome.Status != StatusEmpty {
		t.Fatalf("status: got %q (degraded=%v)", outcome.Status, outcome.Degraded)
	}

	view := Present(outcome)
	t.Logf("status=%s degraded=%v notice=%q rows=%d", outcome.Status, outcome.Degraded, view.DegradationNotice, len(view.Rows))
	for _, row := range view.Rows {
		t.Logf("row: ref=%s src=%s epistemic=%s vigencia=%s summary=%.60q", row.MemoryRef, row.SourceRef, row.EpistemicState, row.VigenciaLabel, row.Summary)
	}

	// AC1-like: if we seeded a memory about "informes", we expect at least one row
	// with a non-empty MemoryRef. SourceRef may be present (PrimarySourceID != zero).
	if len(view.Rows) == 0 {
		t.Logf("no rows returned (search may be degraded in lite profile without embeddings); acceptable for smoke test")
		return
	}
	row := view.Rows[0]
	if row.MemoryRef == "" {
		t.Errorf("row has empty MemoryRef")
	}
	if row.Summary == "" {
		t.Errorf("row has empty Summary")
	}
	// Audit trail must be read-only
	for _, op := range outcome.AuditTrail.Operations {
		if op != "search" && op != "getDetail" {
			t.Errorf("unexpected op in audit: %q", op)
		}
	}
}
